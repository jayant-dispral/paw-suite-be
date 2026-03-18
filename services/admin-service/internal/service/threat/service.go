package threat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	pkgerrors "github.com/jayant-dispral/brand-threat-be/shared/pkg/errors"
	"github.com/jayant-dispral/brand-threat-be/shared/pkg/http/utils"
	"github.com/jayant-dispral/brand-threat-be/shared/ports"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ─────────────────────────────────────────────────────────────────────────────
//  Constants
// ─────────────────────────────────────────────────────────────────────────────

const (
	// scanTimeout is the wall-clock budget for a full scan.
	//
	// FIX: was 4 minutes. With 20k+ permutations the pipeline is:
	//   Stage 1 DNS fan-out:  15–40 s   (100 goroutines × ~20k domains)
	//   Stage 2 enrichment:   60–120 s  (20 workers × ~300 survivors × ~3 s avg)
	//   Persistence:          <5 s
	//   Total comfortable:    ~2–3 min
	//
	// 4 min is fine in the happy path but fails when:
	//   - A DNS resolver rate-limits you (retries push Stage 1 to 90+ s)
	//   - RDAP 429s cause enrichment workers to back off
	//   - The server is under memory pressure and goroutines slow
	//
	// 30 min gives a safety margin while still preventing zombie scans for massive permutation sets.
	scanTimeout = 30 * time.Minute

	// defaultScanFreqHours is the re-scan interval when the project has no
	// explicit configuration.
	defaultScanFreqHours = 24
)

// ─────────────────────────────────────────────────────────────────────────────
//  Types
// ─────────────────────────────────────────────────────────────────────────────

// scanMetrics is the shared counter struct used in both this file and
// analysis.go. Defined here once; analysis.go references it without redefining.
//
// FIX: was duplicated in both files — compile error in same-package builds.
type scanMetrics struct {
	TotalGenerated      int
	ValidCandidates     int
	ProcessedCandidates int
	EnrichedCandidates  int
	SurfacedFindings    int
	Suppressed          int
}

type scanJob struct {
	Fingerprint string
	Cancel      context.CancelFunc
}

type liveScanSnapshot struct {
	SourceDomain      string
	SourceFingerprint string
	Metrics           scanMetrics
	CurrentCandidate  string
	CurrentAlgorithm  string
	Findings          []domain.Threat
}

type ThreatService struct {
	threatRepo    ports.ThreatRepository
	scanStateRepo ports.ThreatScanStateRepository
	projectRepo   ports.ProjectRepository
	mu            sync.Mutex
	inFlight      map[string]scanJob
	liveSnapshots map[string]liveScanSnapshot
}

func NewThreatService(
	threatRepo ports.ThreatRepository,
	scanStateRepo ports.ThreatScanStateRepository,
	projectRepo ports.ProjectRepository,
) ports.ThreatService {
	return &ThreatService{
		threatRepo:    threatRepo,
		scanStateRepo: scanStateRepo,
		projectRepo:   projectRepo,
		inFlight:      make(map[string]scanJob),
		liveSnapshots: make(map[string]liveScanSnapshot),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
//  Public API
// ─────────────────────────────────────────────────────────────────────────────

func (s *ThreatService) GetThreatIntel(ctx context.Context, projectIDStr, userIDStr string) (*ports.ThreatIntelResponse, error) {
	return s.getThreatIntel(ctx, projectIDStr, userIDStr, false)
}

// RefreshThreatIntel forces a new scan to start immediately (if not already running).
// Intended for manual refresh actions from the UI.
func (s *ThreatService) RefreshThreatIntel(ctx context.Context, projectIDStr, userIDStr string) (*ports.ThreatIntelResponse, error) {
	return s.getThreatIntel(ctx, projectIDStr, userIDStr, true)
}

func (s *ThreatService) getThreatIntel(ctx context.Context, projectIDStr, userIDStr string, forceScan bool) (*ports.ThreatIntelResponse, error) {
	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return nil, err
	}
	actorID, err := utils.HexToObjectID(userIDStr)
	if err != nil {
		return nil, err
	}

	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
	}

	// FIX: ownership check was missing. The original parsed userIDStr to an
	// ObjectID and then discarded it — any authenticated caller could read any
	// project's threats by supplying a guessed project ID.
	if project.OwnerID != actorID {
		return nil, pkgerrors.NewError(domain.ErrForbidden, fmt.Errorf("access denied"))
	}

	findings, err := s.threatRepo.FindByProjectID(ctx, projectID)
	if err != nil {
		return nil, err
	}

	scanState := s.loadScanState(ctx, projectID)
	if scanSourceChanged(project, scanState) {
		findings = nil
	} else {
		findings = filterFindingsForDomain(project.PrimaryDomain, findings)
	}

	// FIX: shouldQueueScan is now called under the mutex inside
	// maybeQueueBackgroundScan to close the TOCTOU race window.
	// See maybeQueueBackgroundScan for details.
	s.maybeQueueBackgroundScan(project, scanState, len(findings) == 0, forceScan)
	scanState = s.loadScanState(ctx, projectID)

	metrics := metricsFromState(scanState)
	scan := scanInfo(scanState)

	if snapshot, ok := s.liveSnapshot(project); ok {
		metrics = snapshot.Metrics
		scan = scanInfoWithSnapshot(scanState, snapshot)
		findings = snapshot.Findings
	}

	return &ports.ThreatIntelResponse{
		Summary:  buildSummary(project, metrics, findings),
		Scan:     scan,
		Findings: findings,
	}, nil
}

func (s *ThreatService) ResolveThreat(ctx context.Context, projectIDStr, threatIDStr, userIDStr string) error {
	return s.updateThreatStatus(ctx, projectIDStr, threatIDStr, userIDStr, domain.ThreatResolved)
}

func (s *ThreatService) IgnoreThreat(ctx context.Context, projectIDStr, threatIDStr, userIDStr string) error {
	return s.updateThreatStatus(ctx, projectIDStr, threatIDStr, userIDStr, domain.ThreatFalsePositive)
}

// WhitelistThreat permanently suppresses a threat (10-year TTL via expiryForStatus).
// Use when the flagged domain is legitimately owned by the brand or a known partner —
// it will not surface again unless the project fingerprint changes and a full re-scan
// overwrites the record (mergeThreats preserves the whitelisted status on re-detection).
func (s *ThreatService) WhitelistThreat(ctx context.Context, projectIDStr, threatIDStr, userIDStr string) error {
	return s.updateThreatStatus(ctx, projectIDStr, threatIDStr, userIDStr, domain.ThreatWhitelisted)
}

// AcknowledgeThreat marks the team as aware of a threat without dismissing it.
// The threat remains active and re-surfaces on every scan. ExpiresAt rolls forward
// by 1 month each scan (same as ThreatActive) so it never silently disappears.
// Preserved across re-scans: mergeThreats sees Status != ThreatActive and keeps it.
func (s *ThreatService) AcknowledgeThreat(ctx context.Context, projectIDStr, threatIDStr, userIDStr string) error {
	return s.updateThreatStatus(ctx, projectIDStr, threatIDStr, userIDStr, domain.ThreatAcknowledged)
}

func (s *ThreatService) updateThreatStatus(ctx context.Context, projectIDStr, threatIDStr, userIDStr string, status domain.ThreatStatus) error {
	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return err
	}
	threatID, err := utils.HexToObjectID(threatIDStr)
	if err != nil {
		return err
	}
	actorID, err := utils.HexToObjectID(userIDStr)
	if err != nil {
		return err
	}

	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
	}

	// FIX: ownership check was also missing from updateThreatStatus.
	// The original only verified the threat belonged to the project,
	// but never checked that the calling user owns the project.
	if project.OwnerID != actorID {
		return pkgerrors.NewError(domain.ErrForbidden, fmt.Errorf("access denied"))
	}

	threat, err := s.threatRepo.FindByID(ctx, threatID)
	if err != nil {
		return err
	}
	if threat.ProjectID != projectID {
		return pkgerrors.NewError(domain.ErrForbidden, fmt.Errorf("threat does not belong to this project"))
	}

	return s.threatRepo.UpdateStatus(ctx, threatID, status, actorID)
}

// ─────────────────────────────────────────────────────────────────────────────
//  Scan lifecycle — queueing
// ─────────────────────────────────────────────────────────────────────────────

// maybeQueueBackgroundScan combines the "should I scan?" decision with the
// "start a scan" operation under the same mutex lock.
//
// FIX: the original code called shouldQueueScan() and queueBackgroundScan()
// as separate steps with no lock held between them. Under concurrent requests
// (two API calls arriving within milliseconds), both callers could see
// shouldQueueScan → true and both call queueBackgroundScan, creating two
// goroutines for the same project. The inner duplicate check caught the
// same-fingerprint case, but only after acquiring the mutex — the goroutines
// were already spawned.
//
// Now the decision and the goroutine spawn are both inside one critical section.
func (s *ThreatService) maybeQueueBackgroundScan(project *domain.Project, state *domain.ThreatScanState, emptyCache bool, force bool) {
	if !project.MonitoringConfig.EnableTyposquatting {
		return
	}

	fingerprint := scanFingerprint(project)
	sourceDomain := normalizeDomain(project.PrimaryDomain)

	s.mu.Lock()

	// Is there already an in-flight scan for this exact fingerprint?
	if current, exists := s.inFlight[project.ID.Hex()]; exists && current.Fingerprint == fingerprint {
		s.mu.Unlock()
		return
	}

	// Should we start a scan at all?
	if !force && !s.shouldQueue(project, state, fingerprint, emptyCache) {
		s.mu.Unlock()
		return
	}

	// Cancel any stale in-flight scan for a different fingerprint.
	if current, exists := s.inFlight[project.ID.Hex()]; exists {
		current.Cancel()
	}

	ctx, cancel := context.WithTimeout(context.Background(), scanTimeout)
	s.inFlight[project.ID.Hex()] = scanJob{Fingerprint: fingerprint, Cancel: cancel}
	s.liveSnapshots[project.ID.Hex()] = liveScanSnapshot{
		SourceDomain:      sourceDomain,
		SourceFingerprint: fingerprint,
	}
	s.mu.Unlock()

	now := time.Now().UTC()
	scanState := &domain.ThreatScanState{
		ProjectID:         project.ID,
		Status:            domain.ThreatScanRunning,
		SourceDomain:      sourceDomain,
		SourceFingerprint: fingerprint,
		LastRequestedAt:   &now,
		LastStartedAt:     &now,
		Metrics:           domain.ThreatScanMetrics{},
	}
	if state != nil {
		scanState.CreatedAt = state.CreatedAt
		scanState.LastCompletedAt = state.LastCompletedAt
	}
	if err := s.scanStateRepo.Upsert(context.Background(), scanState); err != nil {
		log.Printf("[ThreatScan] failed to mark scan running for project=%s: %v", project.ID.Hex(), err)
	}

	go s.runBackgroundScan(ctx, cancel, project, now, sourceDomain, fingerprint)
}

// shouldQueue is the decision logic, called only while the mutex is held.
func (s *ThreatService) shouldQueue(
	project *domain.Project,
	state *domain.ThreatScanState,
	fingerprint string,
	emptyCache bool,
) bool {
	if state != nil {
		// Fingerprint mismatch → config changed → must re-scan.
		if state.SourceFingerprint != "" && state.SourceFingerprint != fingerprint {
			return true
		}
		// Legacy records without fingerprint: compare domain strings.
		if state.SourceFingerprint == "" && state.SourceDomain != "" &&
			!sameDomain(state.SourceDomain, project.PrimaryDomain) {
			return true
		}

		// FIX: stale "running" state after a server restart.
		//
		// If the DB says the scan is running but there is no in-flight entry
		// in memory, the previous server process was killed mid-scan. The
		// original code would see status==Running and fingerprint matches →
		// skip re-queuing → project stuck in "running" forever.
		//
		// Fix: if Running but nothing is in-flight, treat as stale and re-queue.
		if state.Status == domain.ThreatScanRunning {
			if _, active := s.inFlight[project.ID.Hex()]; !active {
				log.Printf("[ThreatScan] stale running state detected for project=%s, re-queuing", project.ID.Hex())
				return true
			}
			// Genuinely still running — do not start a duplicate.
			return false
		}
	}

	if emptyCache {
		return true
	}
	if state == nil || state.LastCompletedAt == nil {
		return true
	}

	freqHours := project.MonitoringConfig.TyposquattingScanFreq
	if freqHours <= 0 {
		freqHours = defaultScanFreqHours
	}
	return time.Since(*state.LastCompletedAt) >= time.Duration(freqHours)*time.Hour
}

// ─────────────────────────────────────────────────────────────────────────────
//  Scan lifecycle — execution
// ─────────────────────────────────────────────────────────────────────────────

func (s *ThreatService) runBackgroundScan(
	ctx context.Context,
	cancel context.CancelFunc,
	project *domain.Project,
	startedAt time.Time,
	sourceDomain string,
	fingerprint string,
) {
	projectKey := project.ID.Hex()
	defer cancel()
	defer s.releaseJob(projectKey, fingerprint)

	findings, metrics := analyzeProjectThreats(ctx, project, func(update scanProgressUpdate) {
		s.storeLiveSnapshot(project.ID, liveScanSnapshot{
			SourceDomain:      sourceDomain,
			SourceFingerprint: fingerprint,
			Metrics:           update.Metrics,
			CurrentCandidate:  update.CurrentCandidate,
			CurrentAlgorithm:  update.CurrentAlgorithm,
			Findings:          update.Findings,
		})
	})

	// Explicit cancellation (newer scan started for a different fingerprint).
	// Do not persist — the newer scan will write its own results.
	if errors.Is(ctx.Err(), context.Canceled) {
		return
	}

	if !s.isCurrentJob(projectKey, fingerprint) {
		return
	}

	// FIX: on DeadlineExceeded the original discarded all findings and called
	// markScanFailed(). analyzeProjectThreats had already returned whatever
	// findings it managed to collect before the deadline — wasting all that
	// work. Now we persist partial results and mark the scan as completed
	// (with a log warning) rather than failed.
	timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
	if timedOut {
		log.Printf("[ThreatScan] deadline exceeded for project=%s — persisting %d partial findings", projectKey, len(findings))
	}

	// ── Persist findings ───────────────────────────────────────────────────────
	// Use a fresh background context because the scan context is already expired.
	persistCtx := context.Background()

	existing, err := s.threatRepo.FindByProjectID(persistCtx, project.ID)
	if err != nil {
		s.markScanFailed(project.ID, sourceDomain, fingerprint, err)
		return
	}

	merged := mergeThreats(filterFindingsForDomain(sourceDomain, existing), findings)

	if err := s.threatRepo.DeleteByProjectID(persistCtx, project.ID); err != nil {
		s.markScanFailed(project.ID, sourceDomain, fingerprint, err)
		return
	}
	if len(merged) > 0 {
		if err := s.threatRepo.CreateMany(persistCtx, merged); err != nil {
			s.markScanFailed(project.ID, sourceDomain, fingerprint, err)
			return
		}
	}

	// Mark completed (even if timed-out but partially successful).
	completedAt := time.Now().UTC()
	finalStatus := domain.ThreatScanReady
	if timedOut {
		finalStatus = domain.ThreatScanReady // still surfaced results; not a failure
	}

	state := &domain.ThreatScanState{
		ProjectID:         project.ID,
		Status:            finalStatus,
		SourceDomain:      sourceDomain,
		SourceFingerprint: fingerprint,
		LastRequestedAt:   &startedAt,
		LastStartedAt:     &startedAt,
		LastCompletedAt:   &completedAt,
		Metrics: domain.ThreatScanMetrics{
			TotalGenerated:      metrics.TotalGenerated,
			ValidCandidates:     metrics.ValidCandidates,
			ProcessedCandidates: metrics.ProcessedCandidates,
			EnrichedCandidates:  metrics.EnrichedCandidates,
			SurfacedFindings:    len(merged),
			Suppressed:          metrics.Suppressed,
		},
	}
	if timedOut {
		state.LastError = fmt.Sprintf("scan timed out after %s — %d partial findings persisted", scanTimeout, len(merged))
	}

	if err := s.scanStateRepo.Upsert(context.Background(), state); err != nil {
		log.Printf("[ThreatScan] failed to mark scan ready for project=%s: %v", projectKey, err)
	}

	s.clearLiveSnapshot(project.ID, fingerprint)
}

// ─────────────────────────────────────────────────────────────────────────────
//  In-flight / snapshot management
// ─────────────────────────────────────────────────────────────────────────────

func (s *ThreatService) loadScanState(ctx context.Context, projectID primitive.ObjectID) *domain.ThreatScanState {
	state, err := s.scanStateRepo.GetByProjectID(ctx, projectID)
	if err != nil {
		return nil
	}
	return state
}

func (s *ThreatService) storeLiveSnapshot(projectID primitive.ObjectID, snapshot liveScanSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, exists := s.inFlight[projectID.Hex()]
	if !exists || current.Fingerprint != snapshot.SourceFingerprint {
		return
	}
	s.liveSnapshots[projectID.Hex()] = cloneLiveSnapshot(snapshot)
}

func (s *ThreatService) liveSnapshot(project *domain.Project) (liveScanSnapshot, bool) {
	projectKey := project.ID.Hex()
	expectedFP := scanFingerprint(project)

	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, ok := s.liveSnapshots[projectKey]
	if !ok {
		return liveScanSnapshot{}, false
	}
	if snapshot.SourceFingerprint != expectedFP || !sameDomain(snapshot.SourceDomain, project.PrimaryDomain) {
		return liveScanSnapshot{}, false
	}
	return cloneLiveSnapshot(snapshot), true
}

func (s *ThreatService) clearLiveSnapshot(projectID primitive.ObjectID, fingerprint string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if snapshot, ok := s.liveSnapshots[projectID.Hex()]; ok && snapshot.SourceFingerprint == fingerprint {
		delete(s.liveSnapshots, projectID.Hex())
	}
}

func (s *ThreatService) releaseJob(projectKey, fingerprint string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job, ok := s.inFlight[projectKey]; ok && job.Fingerprint == fingerprint {
		delete(s.inFlight, projectKey)
	}
	if snapshot, ok := s.liveSnapshots[projectKey]; ok && snapshot.SourceFingerprint == fingerprint {
		delete(s.liveSnapshots, projectKey)
	}
}

func (s *ThreatService) isCurrentJob(projectKey, fingerprint string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.inFlight[projectKey]
	return ok && job.Fingerprint == fingerprint
}

func (s *ThreatService) markScanFailed(projectID primitive.ObjectID, sourceDomain, fingerprint string, err error) {
	failedAt := time.Now().UTC()
	state := &domain.ThreatScanState{
		ProjectID:         projectID,
		Status:            domain.ThreatScanFailed,
		SourceDomain:      sourceDomain,
		SourceFingerprint: fingerprint,
		LastCompletedAt:   &failedAt,
		LastError:         err.Error(),
	}
	if upsertErr := s.scanStateRepo.Upsert(context.Background(), state); upsertErr != nil {
		log.Printf("[ThreatScan] failed to mark scan failed for project=%s: %v", projectID.Hex(), upsertErr)
	}
	log.Printf("[ThreatScan] scan failed for project=%s: %v", projectID.Hex(), err)
}

// ─────────────────────────────────────────────────────────────────────────────
//  Response builders
// ─────────────────────────────────────────────────────────────────────────────

func buildSummary(project *domain.Project, metrics scanMetrics, findings []domain.Threat) ports.ThreatIntelSummary {
	active := 0
	critical := 0
	for _, f := range findings {
		if f.Status == domain.ThreatActive {
			active++
		}
		if f.Severity == "critical" {
			critical++
		}
	}

	surfaced := len(findings)
	if metrics.SurfacedFindings > surfaced {
		surfaced = metrics.SurfacedFindings
	}

	return ports.ThreatIntelSummary{
		BrandName:            project.BrandName,
		ProtectedDomain:      project.PrimaryDomain,
		TotalGenerated:       metrics.TotalGenerated,
		ValidCandidates:      metrics.ValidCandidates,
		EnrichedCandidates:   metrics.EnrichedCandidates,
		SuppressedCandidates: metrics.Suppressed,
		SurfacedFindings:     surfaced,
		ActiveFindings:       active,
		CriticalFindings:     critical,
	}
}

func metricsFromState(state *domain.ThreatScanState) scanMetrics {
	if state == nil {
		return scanMetrics{}
	}
	return scanMetrics{
		TotalGenerated:      state.Metrics.TotalGenerated,
		ValidCandidates:     state.Metrics.ValidCandidates,
		ProcessedCandidates: state.Metrics.ProcessedCandidates,
		EnrichedCandidates:  state.Metrics.EnrichedCandidates,
		SurfacedFindings:    state.Metrics.SurfacedFindings,
		Suppressed:          state.Metrics.Suppressed,
	}
}

func scanInfo(state *domain.ThreatScanState) ports.ThreatScanInfo {
	if state == nil {
		return ports.ThreatScanInfo{Status: domain.ThreatScanIdle}
	}
	return ports.ThreatScanInfo{
		Status:              state.Status,
		SourceDomain:        state.SourceDomain,
		ProcessedCandidates: state.Metrics.ProcessedCandidates,
		CurrentCandidate:    state.CurrentCandidate,
		CurrentAlgorithm:    state.CurrentAlgorithm,
		LastRequestedAt:     state.LastRequestedAt,
		LastStartedAt:       state.LastStartedAt,
		LastCompletedAt:     state.LastCompletedAt,
		LastError:           state.LastError,
	}
}

func scanInfoWithSnapshot(state *domain.ThreatScanState, snapshot liveScanSnapshot) ports.ThreatScanInfo {
	info := scanInfo(state)
	if info.Status == domain.ThreatScanIdle {
		info.Status = domain.ThreatScanRunning
	}
	info.SourceDomain = snapshot.SourceDomain
	info.ProcessedCandidates = snapshot.Metrics.ProcessedCandidates
	info.CurrentCandidate = snapshot.CurrentCandidate
	info.CurrentAlgorithm = snapshot.CurrentAlgorithm
	return info
}

// ─────────────────────────────────────────────────────────────────────────────
//  Threat merging
// ─────────────────────────────────────────────────────────────────────────────

// mergeThreats upserts new scan results over the existing stored set.
// User-applied statuses (resolved, false-positive) are preserved.
//
// FIX: ExpiresAt re-detection logic was inverted.
// The original preserved the old ExpiresAt for re-detected threats because it
// was non-zero. A threat resolved 11 months ago that reappears in the new scan
// kept its old 1-month expiry → immediately expired on arrival.
// Fix: always reset ExpiresAt when a threat is re-detected by a fresh scan.
func mergeThreats(existing []domain.Threat, latest []domain.Threat) []domain.Threat {
	now := time.Now().UTC()

	existingByDomain := make(map[string]domain.Threat, len(existing))
	for _, t := range existing {
		if key := strings.ToLower(t.Details.SuspiciousDomain); key != "" {
			existingByDomain[key] = t
		}
	}

	merged := make([]domain.Threat, 0, len(latest))
	for _, threat := range latest {
		key := strings.ToLower(threat.Details.SuspiciousDomain)

		if previous, ok := existingByDomain[key]; ok {
			// Preserve identity and user-authored fields.
			threat.ID = previous.ID
			threat.CreatedAt = previous.CreatedAt
			threat.Notes = previous.Notes

			// Preserve user-applied status transitions (whitelist / resolve / acknowledge).
			// The new scan score/signals always win — the status label survives.
			// ThreatAcknowledged: team is aware but threat is still live — keep the label.
			// ThreatWhitelisted:  defensive registration — never auto-escalate back to active.
			// ThreatResolved/FalsePositive: user dismissed — honour that decision.
			if previous.Status != domain.ThreatActive {
				threat.Status = previous.Status
				threat.AcknowledgedAt = previous.AcknowledgedAt
				threat.AcknowledgedBy = previous.AcknowledgedBy
				threat.ResolvedAt = previous.ResolvedAt
				threat.ResolvedBy = previous.ResolvedBy
			}

			// FIX: always recalculate ExpiresAt for re-detected threats.
			// The old expiry was computed when the previous status was set;
			// it may already be in the past if the threat was re-detected
			// a long time after it was first resolved/whitelisted.
			threat.ExpiresAt = expiryForStatus(threat.Status, now)
		}

		if threat.CreatedAt.IsZero() {
			threat.CreatedAt = now
		}
		threat.UpdatedAt = now
		if threat.ExpiresAt.IsZero() {
			threat.ExpiresAt = expiryForStatus(threat.Status, now)
		}
		merged = append(merged, threat)
	}

	return merged
}

// ─────────────────────────────────────────────────────────────────────────────
//  expiryForStatus — canonical definition (shared with analysis.go)
// ─────────────────────────────────────────────────────────────────────────────
//
// FIX: analysis.go also defined this function with different semantics.
// Canonical version lives here. analysis.go no longer defines it.
//
//   Active         → re-check in 1 month
//   Acknowledged   → re-check in 1 month (still live, team is just aware)
//   Whitelisted    → effectively permanent (10 years)
//   Resolved       → audit trail for 2 months
//   FalsePositive  → audit trail for 2 months
//   default        → 1 month (safe fallback)
func expiryForStatus(status domain.ThreatStatus, now time.Time) time.Time {
	switch status {
	case domain.ThreatActive, domain.ThreatAcknowledged:
		return now.AddDate(0, 1, 0)
	case domain.ThreatWhitelisted:
		return now.AddDate(10, 0, 0)
	case domain.ThreatResolved, domain.ThreatFalsePositive:
		return now.AddDate(0, 2, 0)
	default:
		return now.AddDate(0, 1, 0)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
//  Scan fingerprint & domain helpers
// ─────────────────────────────────────────────────────────────────────────────

// scanFingerprint produces a stable hash of the project fields that affect
// permutation generation. Any change → cache invalidated → new scan queued.
//
// FIX: upgraded from SHA-1 to SHA-256. The hash is used only for equality
// comparison (not cryptographic security), but SHA-256 avoids any future
// concern about theoretical SHA-1 collisions and is negligibly slower.
func scanFingerprint(project *domain.Project) string {
	keywords := append([]string(nil), project.MonitoringConfig.Keywords...)
	sort.Strings(keywords)

	raw := strings.Join([]string{
		normalizeDomain(project.PrimaryDomain),
		strings.ToLower(strings.TrimSpace(project.BrandName)),
		strings.ToLower(strings.TrimSpace(project.Description)),
		strings.Join(normalizeWords(keywords), "|"),
	}, "::")

	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func scanSourceChanged(project *domain.Project, state *domain.ThreatScanState) bool {
	if state == nil {
		return false
	}
	if state.SourceFingerprint != "" {
		return state.SourceFingerprint != scanFingerprint(project)
	}
	if state.SourceDomain != "" {
		return !sameDomain(state.SourceDomain, project.PrimaryDomain)
	}
	return false
}

func filterFindingsForDomain(domainName string, findings []domain.Threat) []domain.Threat {
	if len(findings) == 0 {
		return nil
	}
	filtered := make([]domain.Threat, 0, len(findings))
	for _, f := range findings {
		if f.Details.PrimaryDomain == "" || sameDomain(f.Details.PrimaryDomain, domainName) {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

func sameDomain(left, right string) bool {
	return normalizeDomain(left) == normalizeDomain(right)
}

func normalizeDomain(d string) string {
	return strings.ToLower(strings.TrimSpace(d))
}

func normalizeWords(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if t := strings.ToLower(strings.TrimSpace(v)); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func cloneLiveSnapshot(s liveScanSnapshot) liveScanSnapshot {
	cloned := make([]domain.Threat, len(s.Findings))
	copy(cloned, s.Findings)
	s.Findings = cloned
	return s
}
