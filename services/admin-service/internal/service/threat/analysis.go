package threat

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/idna"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
)

// ─────────────────────────────────────────────────────────────────────────────
//  Package-level vars
// ─────────────────────────────────────────────────────────────────────────────

var (
	tagRe      = regexp.MustCompile(`(?is)<[^>]+>`)
	titleRe    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	passwordRe = regexp.MustCompile(`(?is)type=["']?password["']?`)
	nonWordRe  = regexp.MustCompile(`[^a-z0-9]+`)
	spaceRe    = regexp.MustCompile(`\s+`)

	stopWords = map[string]struct{}{
		"the": {}, "and": {}, "with": {}, "your": {}, "for": {},
		"from": {}, "this": {}, "that": {}, "are": {}, "you": {},
		"our": {}, "www": {}, "com": {}, "net": {}, "org": {},
		"app": {}, "login": {}, "secure": {},
	}

	httpClient = &http.Client{
		Timeout:   8 * time.Second,
		Transport: insecureTransport(),
	}
	httpsDialer = &net.Dialer{Timeout: 5 * time.Second}

	// ── Worker counts ──────────────────────────────────────────────────────
	//
	// FIX: was a single dnsWorkers=100 used for BOTH DNS and enrichment.
	//
	// DNS (Stage 1): 500 concurrent UDP calls — cheap, network-bound.
	// Enrichment (Stage 2): RDAP + SSL dial + HTTP crawl per domain.
	//   100 concurrent = rdap.org returns 429, crawl contexts timeout,
	//   SSL dials exhaust the OS connection table. 50 is a safer ceiling.
	dnsWorkerCount    = 500
	enrichWorkerCount = 50

	// maxNoDNSCandidates caps the number of high-risk candidates that are allowed
	// to proceed to enrichment even when they do NOT resolve A/AAAA records.
	// These are checked via RDAP to catch "registered but dark" domains.
	//
	// TUNED: raised from 400→800 to surface more RDAP-only domains, matching
	// haveibeensquatted's approach of surfacing all registerable lookalikes.
	maxNoDNSCandidates int64 = 800

	// minNoDNSPriority is the minimum algorithm priority to allow a no-DNS
	// candidate into enrichment without A/AAAA resolution.
	minNoDNSPriority = 75

	// minNoDNSLexicalRisk allows very close lookalikes to pass even if their
	// priority is slightly lower.
	minNoDNSLexicalRisk         = 0.85
	minNoDNSPriorityWithLexical = 65

	// minSurfaceScoreDefault is the minimum score for surfacing.
	//
	// TUNED: lowered from 20→1 to surface ALL resolving domains, matching
	// haveibeensquatted's approach. The score is informational for the user,
	// not a gate — any domain that resolves DNS is worth showing.
	minSurfaceScoreDefault = 1
	// minSurfaceScoreRegistered is a lower threshold for registered domains
	// that show DNS signals (MX/NS) or have RDAP registration records.
	minSurfaceScoreRegistered = 1

	// FIX: separate RDAP concurrency limit (20) to avoid excessive 429s from rdap.org.
	// A token bucket or semaphore of 20 is generally within acceptable bursts.
	rdapWorkerCount = 20

	dnsTimeoutFast = 2 * time.Second

	// FIX: dnsResolver used consistently in BOTH Stage 1 and Stage 2.
	// The original lookupDNS() used net.DefaultResolver — inconsistent.
	dnsResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: dnsTimeoutFast}
			return d.DialContext(ctx, network, address)
		},
	}
)

// ─────────────────────────────────────────────────────────────────────────────
//  Types
// ─────────────────────────────────────────────────────────────────────────────

type baselineProfile struct {
	Domain           string
	Label            string
	BrandName        string
	RegistrationDate *time.Time
	Category         string
	Tokens           map[string]struct{}
	BrandTerms       []string
	FuzzyHash        string
}

type rdapProfile struct {
	IsRegistered bool
	Registrar    string
	CreatedAt    *time.Time
	URL          string
}

type webResult struct {
	IsLive            bool
	StatusCode        int
	HTTPBanner        string
	Title             string
	BodyText          string
	HasLoginForm      bool
	LooksLikeBrand    bool
	IsParkingPage     bool
	Technologies      []string
	ContentSimilarity float64
	VisualSimilarity  float64
	FuzzyHash         string
}

// resolvedCandidate carries the permutation PLUS the IPs already resolved in
// Stage 1, so Stage 2 (enrichCandidate) never re-does the DNS A lookup.
//
// FIX: the original code discarded Stage 1 DNS results and called lookupDNS()
// again inside enrichCandidate — doubling every DNS lookup unnecessarily.
type resolvedCandidate struct {
	perm     permutation
	aRecords []string // IPs already resolved; injected into ThreatDNSProfile
}

type candidateResult struct {
	Threat    domain.Threat
	Enriched  bool
	Surfaced  bool
	Domain    string
	Algorithm string
}

type scanProgressUpdate struct {
	Metrics          scanMetrics
	CurrentCandidate string
	CurrentAlgorithm string
	Findings         []domain.Threat
}

// scanMetrics is defined in service.go — same package, no redeclaration needed.

func shouldEnrichWithoutA(candidate permutation) bool {
	if candidate.Priority >= minNoDNSPriority {
		return true
	}
	if candidate.LexicalRisk >= minNoDNSLexicalRisk && candidate.Priority >= minNoDNSPriorityWithLexical {
		return true
	}
	// High-conviction patterns that are worth checking even without A/AAAA.
	switch candidate.Algorithm {
	case "homoglyph", "homoglyph_double", "prefix_tld_combo":
		return true
	default:
		return false
	}
}

// ─────────────────────────────────────────────────────────────────────────────
//  Main entry point
// ─────────────────────────────────────────────────────────────────────────────

func analyzeProjectThreats(
	ctx context.Context,
	project *domain.Project,
	onProgress func(scanProgressUpdate),
) ([]domain.Threat, scanMetrics) {

	metrics := scanMetrics{}
	baseline := buildBaseline(ctx, project)

	// ── Stage 1: generate ALL permutations (pure CPU, ~50 ms) ─────────────────
	all := generatePermutations(project.PrimaryDomain)
	metrics.TotalGenerated = len(all)
	if len(all) == 0 {
		return nil, metrics
	}
	if onProgress != nil {
		onProgress(scanProgressUpdate{Metrics: metrics})
	}

	// ── Stage 2: pre-score & cut — pure CPU, zero network ────────────────────
	shortlisted := preScoreFilter(all)
	if len(shortlisted) == 0 {
		return nil, metrics
	}

	// ── Stages 3+4: DNS and enrichment run as a true pipeline ─────────────────
	// DNS workers feed resolved domains directly into enrichment workers without
	// waiting for all DNS to finish first. This means the first enriched findings
	// appear in ~10s from scan start rather than after the full DNS phase ends.
	candidateCh := make(chan permutation, 256)
	resolvedCh := make(chan resolvedCandidate, 64)
	resultsCh := make(chan candidateResult, 64)

	var (
		dnsProcessed    int64
		validCandidates int64
		noDNSQueued     int64
		cursorDomain    atomic.Value
		cursorAlgorithm atomic.Value
	)
	cursorDomain.Store("")
	cursorAlgorithm.Store("")

	// Feed permutations into DNS workers.
	go func() {
		defer close(candidateCh)
		for _, c := range shortlisted {
			select {
			case candidateCh <- c:
			case <-ctx.Done():
				return
			}
		}
	}()

	// DNS workers — 100 concurrent lightweight UDP checks.
	var dnsWG sync.WaitGroup
	for i := 0; i < dnsWorkerCount; i++ {
		dnsWG.Add(1)
		go func() {
			defer dnsWG.Done()
			for candidate := range candidateCh {
				if ctx.Err() != nil {
					return
				}
				dnsCtx, cancel := context.WithTimeout(ctx, dnsTimeoutFast)
				ips, ok := quickDNSCheck(dnsCtx, candidate.Domain)
				cancel()

				atomic.AddInt64(&dnsProcessed, 1)
				cursorDomain.Store(candidate.Domain)
				cursorAlgorithm.Store(candidate.Algorithm)

				if ok {
					atomic.AddInt64(&validCandidates, 1)
					select {
					case resolvedCh <- resolvedCandidate{perm: candidate, aRecords: ips}:
					case <-ctx.Done():
						return
					}
					continue
				}

				// Allow a limited number of high-risk candidates through even without A/AAAA.
				if !shouldEnrichWithoutA(candidate) {
					continue
				}
				if atomic.AddInt64(&noDNSQueued, 1) > maxNoDNSCandidates {
					atomic.AddInt64(&noDNSQueued, -1)
					continue
				}
				select {
				case resolvedCh <- resolvedCandidate{perm: candidate, aRecords: nil}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		dnsWG.Wait()
		close(resolvedCh)
	}()

	// Enrichment workers — 20 concurrent RDAP+SSL+web crawls.
	rdapSem := make(chan struct{}, rdapWorkerCount)
	var enrichWG sync.WaitGroup
	for i := 0; i < enrichWorkerCount; i++ {
		enrichWG.Add(1)
		go func() {
			defer enrichWG.Done()
			for rc := range resolvedCh {
				if ctx.Err() != nil {
					return
				}
				select {
				case resultsCh <- enrichCandidate(ctx, project, baseline, rc, rdapSem):
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		enrichWG.Wait()
		close(resultsCh)
	}()

	// ── Collect & stream findings ─────────────────────────────────────────────
	allFindings := make([]domain.Threat, 0, 64)

	// heartbeatTicker fires every 750ms to emit progress even during the DNS
	// phase when no enrichment results are arriving yet.
	heartbeat := time.NewTicker(750 * time.Millisecond)
	defer heartbeat.Stop()

	emitProgress := func(domain, algorithm string) {
		if onProgress == nil {
			return
		}
		snap := cloneThreats(allFindings)
		sortThreats(snap)
		if len(snap) > 50 {
			snap = snap[:50]
		}
		metrics.ProcessedCandidates = int(atomic.LoadInt64(&dnsProcessed))
		metrics.ValidCandidates = int(atomic.LoadInt64(&validCandidates))
		metrics.SurfacedFindings = len(allFindings)
		metrics.Suppressed = maxInt(0, metrics.EnrichedCandidates-len(allFindings))
		onProgress(scanProgressUpdate{
			Metrics:          metrics,
			CurrentCandidate: domain,
			CurrentAlgorithm: algorithm,
			Findings:         snap,
		})
	}

	for {
		select {
		case result, ok := <-resultsCh:
			if !ok {
				goto done
			}
			if result.Enriched {
				metrics.EnrichedCandidates++
			}
			if result.Surfaced {
				allFindings = append(allFindings, result.Threat)
			}
			emitProgress(result.Domain, result.Algorithm)

		case <-heartbeat.C:
			d, _ := cursorDomain.Load().(string)
			a, _ := cursorAlgorithm.Load().(string)
			emitProgress(d, a)
		}
	}
done:
	metrics.ProcessedCandidates = int(atomic.LoadInt64(&dnsProcessed))
	metrics.ValidCandidates = int(atomic.LoadInt64(&validCandidates))

	if ctx.Err() != nil {
		return nil, metrics
	}

	sortThreats(allFindings)
	// TUNED: raised from 500→1000 to match haveibeensquatted's volume
	// (typically 200–600 results per domain scan).
	if len(allFindings) > 1000 {
		allFindings = allFindings[:1000]
	}
	metrics.SurfacedFindings = len(allFindings)
	metrics.Suppressed = maxInt(0, metrics.EnrichedCandidates-len(allFindings))
	return allFindings, metrics
}

// fastDNSFilter is kept for any callers that need the slice form.

func fastDNSFilterStreaming(
	ctx context.Context,
	candidates []permutation,
	resolvedCh chan<- resolvedCandidate,
	onResolved func(),
) {
	if len(candidates) == 0 {
		close(resolvedCh)
		return
	}

	sem := make(chan struct{}, dnsWorkerCount)
	var wg sync.WaitGroup

	for i := range candidates {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			dnsCtx, cancel := context.WithTimeout(ctx, dnsTimeoutFast)
			defer cancel()

			ips, ok := quickDNSCheck(dnsCtx, candidates[i].Domain)
			if ok {
				resolvedCh <- resolvedCandidate{
					perm:     candidates[i],
					aRecords: ips,
				}
				// onResolved is called without a lock — the caller is responsible
				// for any synchronisation it needs (e.g. atomic increment).
				// Calling it under a mutex here would serialise all 100 DNS
				// goroutines on every hit, defeating the concurrency.
				onResolved()
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resolvedCh)
	}()
}

// fastDNSFilter is kept for any callers that need the slice form.
// Internally delegates to fastDNSFilterStreaming.
func fastDNSFilter(ctx context.Context, candidates []permutation) []resolvedCandidate {
	ch := make(chan resolvedCandidate, 256)
	go fastDNSFilterStreaming(ctx, candidates, ch, func() {})
	var out []resolvedCandidate
	for rc := range ch {
		out = append(out, rc)
	}
	return out
}

// quickDNSCheck returns the resolved IPs and true if the domain has an A record.
//
// FIX: homoglyph domains are stored as Unicode in permutation_v2.go
// (e.g. "аcmecorp.com" with Cyrillic 'а'). net.Resolver cannot resolve
// unicode labels directly — it needs punycode (xn--cmecorp-e3a.com).
// Without idna.ToASCII() the entire homoglyph algorithm (~2,448 variants
// for an 8-char domain) silently returns false and is thrown away.
func quickDNSCheck(ctx context.Context, domainName string) ([]string, bool) {
	ascii, err := idna.ToASCII(domainName)
	if err != nil {
		ascii = domainName
	}

	addrs, err := dnsResolver.LookupIPAddr(ctx, ascii)
	if err != nil || len(addrs) == 0 {
		return nil, false
	}

	ips := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		if v4 := addr.IP.To4(); v4 != nil {
			ips = append(ips, v4.String())
		} else {
			ips = append(ips, addr.IP.String())
		}
	}
	return ips, true
}

// ─────────────────────────────────────────────────────────────────────────────
//  Stage 2 — full enrichment
// ─────────────────────────────────────────────────────────────────────────────

// enrichCandidate runs RDAP + SSL + web crawl on a single confirmed-live domain.
func enrichCandidate(
	ctx context.Context,
	project *domain.Project,
	baseline baselineProfile,
	rc resolvedCandidate,
	rdapSem chan struct{},
) candidateResult {

	candidate := rc.perm
	candidateCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	// ── DNS profile — inject Stage 1 IPs, then MX/NS/TXT lookup only ────────
	dnsProfile := lookupDNSPartial(candidateCtx, candidate.Domain, rc.aRecords)

	// ── RDAP — rate-limited and context-aware ────────────────────────────────
	//
	// FIX: the original used a bare channel send:
	//   rdapSem <- struct{}{}
	// This blocks indefinitely waiting for a slot, ignoring the candidateCtx
	// deadline entirely. With 300+ enrichment goroutines all queuing for 10
	// RDAP slots, goroutines whose candidateCtx (20s) had already expired were
	// still blocking, holding the enrichSem slot and preventing new goroutines
	// from starting. Under sustained load this caused the entire enrichment
	// pipeline to stall well past the scan deadline.
	//
	// Fix: use a select so the goroutine exits the RDAP wait as soon as either
	// a slot becomes available OR the context is cancelled/expired.
	// If the context fires first we skip RDAP but continue with DNS-only scoring
	// — the domain still has A records and potentially MX/NS data worth scoring.
	rdap := rdapProfile{URL: "https://rdap.org/domain/" + candidate.Domain}
	shouldQueryRDAP := dnsProfile.Resolves || dnsProfile.HasMX ||
		len(dnsProfile.NSRecords) > 0 || candidate.Priority >= 75
	if shouldQueryRDAP {
		select {
		case rdapSem <- struct{}{}:
			rdap = queryRDAP(candidateCtx, candidate.Domain)
			<-rdapSem
		case <-candidateCtx.Done():
			// Context expired or cancelled while waiting for an RDAP slot.
			// Continue without RDAP data — DNS profile is sufficient for scoring.
		}
	}

	// If nothing is live even after RDAP, skip enrichment entirely.
	if !rdap.IsRegistered && !dnsProfile.Resolves &&
		!dnsProfile.HasMX && len(dnsProfile.NSRecords) == 0 {
		return candidateResult{Domain: candidate.Domain, Algorithm: candidate.Algorithm}
	}

	// ── SSL + web crawl ──────────────────────────────────────────────────────
	// If the domain doesn't resolve to an IP, skip live network probes.
	sslProfile := domain.ThreatSSLProfile{}
	web := domain.ThreatWebProfile{}
	webText := ""
	if dnsProfile.Resolves {
		sslProfile = inspectSSL(candidateCtx, candidate.Domain)
		web, webText, _ = inspectWeb(candidateCtx, candidate.Domain, baseline.BrandTerms, baseline.Tokens, baseline.FuzzyHash)
	}

	// ── Category classification ──────────────────────────────────────────────
	categoryText := web.Title + " " + webText
	candidateCategory, categoryConfidence := classifyCategoryFromText(categoryText)
	categorySimilarity := categorySimilarityScore(baseline.Category, candidateCategory)
	matchesCategory := categorySimilarity >= 70

	// ── Content similarity ───────────────────────────────────────────────────
	//
	// FIX: the original code computed tokenSimilarity (Jaccard of brand tokens
	// vs page tokens) then OVERWROTE it with lexicalSimilarity(label, label).
	// The label-only score ignores the page content entirely.
	//
	// Correct order of precedence:
	//   1. Jaccard of brand tokens vs page tokens — best signal for live sites.
	//   2. LexicalRisk from permutation_v2 (Jaro-Winkler, 0–1) — good for
	//      unregistered / no-content domains where there is no page to crawl.
	//   Keep the higher of the two so neither discards the other's signal.
	tokenSimilarity := jaccardScore(baseline.Tokens, tokenSet(categoryText))
	labelSimilarity := candidate.LexicalRisk * 100 // Jaro-Winkler 0..1 → 0..100
	contentSimilarity := math.Max(tokenSimilarity, labelSimilarity)
	web.ContentSimilarity = roundFloat(contentSimilarity)

	if web.VisualSimilarity < web.ContentSimilarity {
		web.VisualSimilarity = roundFloat(
			minFloat(100, web.ContentSimilarity+brandBoost(web.LooksLikeBrand)),
		)
	}

	// ── Score ────────────────────────────────────────────────────────────────
	score, threatType, signals, olderThanPrimary, ageDeltaDays :=
		scoreThreat(project, baseline, candidate, rdap, dnsProfile, sslProfile, web, candidateCategory, categorySimilarity)

	minScore := minSurfaceScoreDefault
	if rdap.IsRegistered || dnsProfile.HasMX || len(dnsProfile.NSRecords) > 0 {
		minScore = minSurfaceScoreRegistered
	}
	if score < minScore {
		return candidateResult{
			Enriched:  true,
			Domain:    candidate.Domain,
			Algorithm: candidate.Algorithm,
		}
	}

	// If a domain resolves in DNS it IS squatted — surface it regardless of
	// whether it has live web content, MX, or SSL. Those signals affect the
	// score and severity displayed to the user, but they are not existence gates.
	//
	// Additionally, we now allow registered-but-dark domains to surface at a
	// lower threshold if RDAP confirms registration or DNS shows MX/NS records.
	// This aligns closer to industry tools that list registered squats even
	// when they don't resolve A/AAAA.

	// ── Build Threat record ──────────────────────────────────────────────────
	registrationDate := rdap.CreatedAt
	if registrationDate == nil {
		now := time.Now().UTC()
		registrationDate = &now
	}

	threat := domain.Threat{
		ProjectID: project.ID,
		Type:      threatType,
		Status:    domain.ThreatActive,
		Severity:  severityFromScore(score),
		Score:     score,
		Summary:   buildThreatSummary(candidate.Domain, threatType, score, web, dnsProfile),
		Details: domain.ThreatDetails{
			SuspiciousDomain: candidate.Domain,
			Registrar:        rdap.Registrar,
			RegistrationDate: registrationDate,
			DNSRecords:       dnsLegacyMap(dnsProfile),
			SimilarityScore:  web.ContentSimilarity,
			Description:      buildThreatDescription(project, candidate, web, candidateCategory, olderThanPrimary),
			PrimaryDomain:    project.PrimaryDomain,
			PermutationType:  candidate.Algorithm,
			RDAPURL:          rdap.URL,
			AgeDeltaDays:     ageDeltaDays,
			OlderThanPrimary: olderThanPrimary,
			DNS:              dnsProfile,
			SSL:              sslProfile,
			Web:              web,
			Business: domain.ThreatBusinessProfile{
				TargetCategory:    baseline.Category,
				CandidateCategory: candidateCategory,
				Confidence:        roundFloat(categoryConfidence),
				Similarity:        roundFloat(categorySimilarity),
				MatchesTarget:     matchesCategory,
			},
			Signals:            signals,
			RecommendedActions: recommendedActions(candidate.Domain, threatType, score, dnsProfile.HasMX, web.HasLoginForm, olderThanPrimary, rdap.Registrar),
		},
		DetectedAt: detectedAt(registrationDate),
		ExpiresAt:  expiryForStatus(domain.ThreatActive, time.Now().UTC()),
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}

	return candidateResult{
		Threat:    threat,
		Enriched:  true,
		Surfaced:  true,
		Domain:    candidate.Domain,
		Algorithm: candidate.Algorithm,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
//  Baseline
// ─────────────────────────────────────────────────────────────────────────────

func buildBaseline(ctx context.Context, project *domain.Project) baselineProfile {
	label, _ := splitDomain(project.PrimaryDomain)
	terms := brandTerms(project)
	tokens := tokenSet(
		project.BrandName + " " + label + " " +
			project.Description + " " +
			strings.Join(project.MonitoringConfig.Keywords, " "),
	)

	rdap := queryRDAP(ctx, project.PrimaryDomain)
	webProfile, webText, fuzzyHash := inspectWeb(ctx, project.PrimaryDomain, terms, tokens, "")
	for token := range tokenSet(webProfile.Title + " " + webText) {
		tokens[token] = struct{}{}
	}

	category, _ := classifyCategoryFromText(webText + " " + webProfile.Title)
	if category == "unknown" {
		category = classifyProjectCategory(project)
	}

	return baselineProfile{
		Domain:           project.PrimaryDomain,
		Label:            label,
		BrandName:        project.BrandName,
		RegistrationDate: rdap.CreatedAt,
		Category:         category,
		Tokens:           tokens,
		BrandTerms:       terms,
		FuzzyHash:        fuzzyHash,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
//  DNS helpers
// ─────────────────────────────────────────────────────────────────────────────

// lookupDNSPartial builds a ThreatDNSProfile from Stage 1 IPs (already resolved)
// plus a fresh MX/NS/TXT lookup. It never re-does the A-record lookup.
func lookupDNSPartial(ctx context.Context, domainName string, aRecords []string) domain.ThreatDNSProfile {
	ascii, err := idna.ToASCII(domainName)
	if err != nil {
		ascii = domainName
	}

	profile := domain.ThreatDNSProfile{
		ARecords: aRecords,
		Resolves: len(aRecords) > 0,
	}

	if len(aRecords) > 0 {
		profile.GeoLocation = ResolveGeoIP(ctx, aRecords[0])
	}

	if mxRecs, err := dnsResolver.LookupMX(ctx, ascii); err == nil {
		for _, mx := range mxRecs {
			profile.MXRecords = appendIfMissing(profile.MXRecords, strings.TrimSuffix(mx.Host, "."))
		}
	}
	if nsRecs, err := dnsResolver.LookupNS(ctx, ascii); err == nil {
		for _, ns := range nsRecs {
			profile.NSRecords = appendIfMissing(profile.NSRecords, strings.TrimSuffix(ns.Host, "."))
		}
	}
	if txtRecs, err := dnsResolver.LookupTXT(ctx, ascii); err == nil {
		for _, txt := range txtRecs {
			profile.TXTRecords = appendIfMissing(profile.TXTRecords, txt)
		}
	}

	profile.HasMX = len(profile.MXRecords) > 0
	return profile
}

// ─────────────────────────────────────────────────────────────────────────────
//  RDAP
// ─────────────────────────────────────────────────────────────────────────────

func queryRDAP(ctx context.Context, domainName string) rdapProfile {
	profile := rdapProfile{URL: "https://rdap.org/domain/" + domainName}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, profile.URL, nil)
	if err != nil {
		return profile
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return profile
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound ||
		resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return profile
	}

	var payload map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return profile
	}

	profile.IsRegistered = true
	profile.Registrar = extractRegistrar(payload)
	profile.CreatedAt = extractRegistrationTime(payload)
	return profile
}

// ─────────────────────────────────────────────────────────────────────────────
//  SSL
// ─────────────────────────────────────────────────────────────────────────────

func inspectSSL(ctx context.Context, domainName string) domain.ThreatSSLProfile {
	profile := domain.ThreatSSLProfile{}

	conn, err := tls.DialWithDialer(httpsDialer, "tcp",
		net.JoinHostPort(domainName, "443"),
		&tls.Config{ServerName: domainName, InsecureSkipVerify: true},
	)
	if err != nil {
		return profile
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return profile
	}

	cert := state.PeerCertificates[0]
	profile.HasCertificate = true
	profile.Issuer = cert.Issuer.CommonName
	profile.CommonName = cert.Subject.CommonName
	validFrom := cert.NotBefore.UTC()
	validTo := cert.NotAfter.UTC()
	profile.ValidFrom = &validFrom
	profile.ValidTo = &validTo
	return profile
}

// ─────────────────────────────────────────────────────────────────────────────
//  Web
// ─────────────────────────────────────────────────────────────────────────────

func inspectWeb(
	ctx context.Context,
	domainName string,
	brandTerms []string,
	baselineTokens map[string]struct{},
	baselineFuzzy string,
) (domain.ThreatWebProfile, string, string) {

	raw := fetchWeb(ctx, domainName)
	contentTokens := tokenSet(raw.Title + " " + raw.BodyText)
	contentSimilarity := jaccardScore(baselineTokens, contentTokens)
	looksLikeBrand := raw.LooksLikeBrand ||
		containsAny(strings.ToLower(raw.Title+" "+raw.BodyText), brandTerms)
	visualSimilarity := roundFloat(minFloat(100, contentSimilarity+brandBoost(looksLikeBrand)))

	fuzzyMatch := 0.0
	if baselineFuzzy != "" && raw.FuzzyHash != "" {
		fuzzyMatch = roundFloat(CompareFuzzyHash(baselineFuzzy, raw.FuzzyHash))
	}

	return domain.ThreatWebProfile{
		IsLive:            raw.IsLive,
		HTTPStatus:        raw.StatusCode,
		HTTPBanner:        raw.HTTPBanner,
		Title:             raw.Title,
		HasLoginForm:      raw.HasLoginForm,
		LooksLikeBrand:    looksLikeBrand,
		IsParkingPage:     raw.IsParkingPage,
		Technologies:      raw.Technologies,
		VisualSimilarity:  visualSimilarity,
		ContentSimilarity: roundFloat(contentSimilarity),
		FuzzyMatchScore:   fuzzyMatch,
	}, raw.BodyText, raw.FuzzyHash
}

func fetchWeb(ctx context.Context, domainName string) webResult {
	for _, scheme := range []string{"https://", "http://"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, scheme+domainName, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; BrandScanner/1.0)")

		resp, err := httpClient.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
		resp.Body.Close()

		title := extractTitle(string(body))
		text := cleanText(string(body))
		lower := strings.ToLower(string(body) + " " + title + " " + text)
		fuzzyHash := CalculateFuzzyHash(body)

		return webResult{
			IsLive:         resp.StatusCode > 0,
			StatusCode:     resp.StatusCode,
			HTTPBanner:     extractHTTPBanner(resp.Header),
			Title:          title,
			BodyText:       text,
			HasLoginForm:   passwordRe.MatchString(lower) || strings.Contains(lower, "sign in") || strings.Contains(lower, "log in"),
			IsParkingPage:  strings.Contains(lower, "buy this domain") || strings.Contains(lower, "parked free") || strings.Contains(lower, "domain for sale") || strings.Contains(lower, "sedo"),
			LooksLikeBrand: false,
			Technologies:   detectTechnologies(resp.Header, lower),
			FuzzyHash:      fuzzyHash,
		}
	}
	return webResult{}
}

// ─────────────────────────────────────────────────────────────────────────────
//  Scoring
// ─────────────────────────────────────────────────────────────────────────────

func scoreThreat(
	project *domain.Project,
	baseline baselineProfile,
	candidate permutation,
	rdap rdapProfile,
	dnsProfile domain.ThreatDNSProfile,
	sslProfile domain.ThreatSSLProfile,
	web domain.ThreatWebProfile,
	candidateCategory string,
	categorySimilarity float64,
) (int, domain.ThreatType, []domain.ThreatSignal, bool, int) {

	signals := make([]domain.ThreatSignal, 0, 14)

	// ── Base score ────────────────────────────────────────────────────────────
	labelScore := int(candidate.LexicalRisk * 30)
	priorityScore := candidate.Priority / 5
	score := minInt(40, labelScore+priorityScore)

	// ── Proximity bonus ─────────────────────────────────────────────────────
	// Domains that are 1-2 edits away from the primary domain are the most
	// dangerous for phishing — they are nearly indistinguishable from the
	// real domain. Give them a significant score boost so they rank above
	// prefix/suffix variants that may have more enrichment signals.
	editDist := levenshteinDistance(baseline.Label, candidate.Label)
	if editDist == 1 {
		signals = append(signals, domain.ThreatSignal{
			Signal: "close_domain_mutation", Weight: 25,
			Detail: "Domain is only 1 character edit away from the protected domain — extremely confusable.",
		})
		score += 25
	} else if editDist == 2 {
		signals = append(signals, domain.ThreatSignal{
			Signal: "close_domain_mutation", Weight: 15,
			Detail: "Domain is only 2 character edits away from the protected domain — highly confusable.",
		})
		score += 15
	}

	// ── Age signal ───────────────────────────────────────────────────────────
	olderThanPrimary := false
	ageDeltaDays := 0
	if baseline.RegistrationDate != nil && rdap.CreatedAt != nil {
		ageDeltaDays = int(rdap.CreatedAt.Sub(*baseline.RegistrationDate).Hours() / 24)
		olderThanPrimary = ageDeltaDays < -30
		if olderThanPrimary {
			signals = append(signals, domain.ThreatSignal{
				Signal: "older_than_primary", Weight: -60,
				Detail: "The candidate domain predates the protected domain by more than 30 days.",
			})
			score -= 60
		} else if ageDeltaDays > 30 {
			signals = append(signals, domain.ThreatSignal{
				Signal: "registered_after_primary", Weight: 15,
				Detail: "The candidate domain was registered after the protected domain.",
			})
			score += 15
		}
	}

	// ── Recently registered — strong phishing indicator ───────────────────────
	// Phishing domains are typically registered days to weeks before a campaign.
	// A lookalike registered within the last 12 months is significantly more
	// likely to be malicious than one registered years ago.
	if rdap.CreatedAt != nil {
		ageMonths := int(time.Since(*rdap.CreatedAt).Hours() / (24 * 30))
		if ageMonths <= 3 {
			signals = append(signals, domain.ThreatSignal{
				Signal: "very_recently_registered", Weight: 20,
				Detail: fmt.Sprintf("Domain registered only %d month(s) ago — high-risk indicator.", ageMonths),
			})
			score += 20
		} else if ageMonths <= 12 {
			signals = append(signals, domain.ThreatSignal{
				Signal: "recently_registered", Weight: 10,
				Detail: fmt.Sprintf("Domain registered %d months ago — within the high-risk window.", ageMonths),
			})
			score += 10
		}
	}

	// ── DNS signals ──────────────────────────────────────────────────────────
	if dnsProfile.HasMX {
		isRogue := false
		rogueMXList := []string{"parkingcore.com", "sedoparking.com", "hushmail.com", "bodis.com"}
		for _, mx := range dnsProfile.MXRecords {
			lowerMX := strings.ToLower(mx)
			for _, rogue := range rogueMXList {
				if strings.Contains(lowerMX, rogue) {
					isRogue = true
					break
				}
			}
		}

		if isRogue {
			signals = append(signals, domain.ThreatSignal{
				Signal: "rogue_mx_records", Weight: 45,
				Detail: "The domain uses a known parked or suspicious Mail Exchange provider.",
			})
			score += 45
		} else {
			signals = append(signals, domain.ThreatSignal{
				Signal: "has_mx_records", Weight: 30,
				Detail: "The domain can receive email — primary phishing vector.",
			})
			score += 30
		}
	}
	if dnsProfile.Resolves {
		signals = append(signals, domain.ThreatSignal{
			Signal: "dns_resolves", Weight: 8,
			Detail: "The domain resolves to a live IP address.",
		})
		score += 8
	}
	if len(dnsProfile.NSRecords) > 0 {
		signals = append(signals, domain.ThreatSignal{
			Signal: "has_ns_records", Weight: 4,
			Detail: "Authoritative name servers are configured.",
		})
		score += 4
	}

	// ── SSL signal ───────────────────────────────────────────────────────────
	if sslProfile.HasCertificate {
		signals = append(signals, domain.ThreatSignal{
			Signal: "ssl_enabled", Weight: 5,
			Detail: "TLS is enabled, making the domain look trustworthy to victims.",
		})
		score += 5
	}

	// ── Web content signals ───────────────────────────────────────────────────
	if web.IsLive {
		signals = append(signals, domain.ThreatSignal{
			Signal: "live_web_content", Weight: 8,
			Detail: "HTTP content is being served from this domain.",
		})
		score += 8
	}
	if web.HasLoginForm {
		signals = append(signals, domain.ThreatSignal{
			Signal: "login_form_detected", Weight: 28,
			Detail: "A password or sign-in flow is present on the page.",
		})
		score += 28
	}
	if web.LooksLikeBrand {
		signals = append(signals, domain.ThreatSignal{
			Signal: "brand_language_detected", Weight: 20,
			Detail: "The live content references the protected brand or similar terms.",
		})
		score += 20
	}
	if web.FuzzyMatchScore > 60 {
		fuzzyBoost := 20
		fuzzyLabel := "moderately"
		if web.FuzzyMatchScore > 80 {
			fuzzyBoost = 45
			fuzzyLabel = "strongly"
		}
		signals = append(signals, domain.ThreatSignal{
			Signal: "fuzzy_hash_match", Weight: fuzzyBoost,
			Detail: fmt.Sprintf("HTML structural hash %s matches the protected brand (%.0f%%).", fuzzyLabel, web.FuzzyMatchScore),
		})
		score += fuzzyBoost
	}
	if web.IsParkingPage {
		signals = append(signals, domain.ThreatSignal{
			Signal: "parking_page", Weight: -20,
			Detail: "The content looks like a parked or for-sale landing page.",
		})
		score -= 20
	}

	if web.ContentSimilarity > 0 {
		contentBoost := minInt(35, int(web.ContentSimilarity/3))
		if contentBoost > 0 {
			signals = append(signals, domain.ThreatSignal{
				Signal: "content_similarity", Weight: contentBoost,
				Detail: fmt.Sprintf("Content similarity to the protected brand is %.0f%%.", web.ContentSimilarity),
			})
			score += contentBoost
		}
	}

	// ── Business category signals ─────────────────────────────────────────────
	sameCategory := false
	if categorySimilarity >= 70 {
		sameCategory = true
		signals = append(signals, domain.ThreatSignal{
			Signal: "same_business_category", Weight: 25,
			Detail: fmt.Sprintf("Same business category as the protected brand (%s).", baseline.Category),
		})
		score += 25
	} else if candidateCategory != "unknown" {
		signals = append(signals, domain.ThreatSignal{
			Signal: "different_business_category", Weight: -14,
			Detail: fmt.Sprintf("Page looks like %s, not %s.", candidateCategory, baseline.Category),
		})
		score -= 14
	}

	// ── Triple-threat multiplier ──────────────────────────────────────────────
	// When a domain has ALL three danger signals simultaneously — similar name,
	// similar webpage, AND same business — it is almost certainly an active
	// impersonation/phishing operation. Apply a significant bonus.
	similarDomain := candidate.LexicalRisk >= 0.6
	similarWebpage := web.ContentSimilarity >= 50 || web.LooksLikeBrand || web.FuzzyMatchScore > 60
	if similarDomain && similarWebpage && sameCategory {
		signals = append(signals, domain.ThreatSignal{
			Signal: "triple_threat_impersonation", Weight: 30,
			Detail: "CRITICAL: Similar domain + similar webpage + same business category — active impersonation.",
		})
		score += 30
	}

	// ── Registrar signals ─────────────────────────────────────────────────────
	if trustedRegistrar(rdap.Registrar) {
		signals = append(signals, domain.ThreatSignal{
			Signal: "trusted_registrar", Weight: -40,
			Detail: "Registered via a corporate brand-protection registrar — likely defensive.",
		})
		score -= 40
	}

	// ── Algorithm-specific boosts ─────────────────────────────────────────────
	switch candidate.Algorithm {
	case "homoglyph", "homoglyph_double":
		signals = append(signals, domain.ThreatSignal{
			Signal: "homoglyph_technique", Weight: 30,
			Detail: "CRITICAL: IDN Homograph attack. Uses characters from other scripts (e.g. Cyrillic/Greek) to visually spoof the brand.",
		})
		score += 30
	case "prefix_tld_combo":
		signals = append(signals, domain.ThreatSignal{
			Signal: "phishing_template", Weight: 10,
			Detail: "Matches a known phishing URL pattern (prefix-brand.tld).",
		})
		score += 10
	}

	score = maxInt(0, minInt(100, score))
	threatType := classifyThreatType(score, web, dnsProfile)
	if threatType == domain.ThreatPhishing {
		score = maxInt(score, 65)
	}

	return score, threatType, signals, olderThanPrimary, ageDeltaDays
}

// ─────────────────────────────────────────────────────────────────────────────
//  Classification helpers
// ─────────────────────────────────────────────────────────────────────────────

func classifyThreatType(score int, web domain.ThreatWebProfile, dns domain.ThreatDNSProfile) domain.ThreatType {
	// Triple-threat: high score with brand-like content = phishing even without login form.
	switch {
	case web.HasLoginForm || (dns.HasMX && web.LooksLikeBrand && score >= 70):
		return domain.ThreatPhishing
	case score >= 85 && (web.LooksLikeBrand || web.ContentSimilarity >= 50):
		// Active impersonation with very high confidence → treat as phishing
		return domain.ThreatPhishing
	case web.LooksLikeBrand || score >= 70:
		return domain.ThreatImpersonation
	default:
		return domain.ThreatTyposquatting
	}
}

func classifyProjectCategory(project *domain.Project) string {
	text := project.BrandName + " " + project.Description + " " +
		strings.Join(project.MonitoringConfig.Keywords, " ")
	category, _ := classifyCategoryFromText(text)
	if category == "unknown" {
		return "software & saas"
	}
	return category
}

func classifyCategoryFromText(text string) (string, float64) {
	keywords := map[string][]string{
		"financial services":    {"bank", "loan", "credit", "payment", "wallet", "finance", "insurance", "invest", "mortgage", "upi", "remittance", "fintech"},
		"security software":     {"security", "threat", "monitor", "phishing", "intel", "detection", "incident", "vulnerability", "siem", "firewall", "antivirus"},
		"software & saas":       {"platform", "software", "dashboard", "workspace", "automation", "saas", "api", "integration", "workflow", "devops", "cloud"},
		"healthcare":            {"health", "patient", "doctor", "clinic", "medical", "care", "hospital", "pharmacy", "treatment", "wellness", "diagnostic"},
		"e-commerce":            {"shop", "checkout", "cart", "store", "product", "shipping", "order", "merchant", "retail", "buy", "price", "catalog"},
		"education":             {"course", "learn", "student", "training", "academy", "education", "university", "certification", "tutor", "exam"},
		"crypto & web3":         {"crypto", "bitcoin", "ethereum", "wallet", "blockchain", "defi", "nft", "token", "exchange", "mining", "staking"},
		"logistics":             {"shipping", "freight", "logistics", "delivery", "tracking", "cargo", "parcel", "courier", "warehouse", "supply chain"},
		"media & entertainment": {"news", "video", "stream", "music", "movie", "entertainment", "podcast", "magazine", "content", "media"},
		"travel & hospitality":  {"travel", "hotel", "flight", "booking", "vacation", "resort", "airline", "tourism", "hostel", "cruise"},
		"real estate":           {"property", "real estate", "apartment", "rent", "lease", "housing", "mortgage", "realtor", "estate", "listing"},
		"food & delivery":       {"food", "restaurant", "delivery", "menu", "order", "recipe", "cuisine", "dining", "takeout", "grocery"},
		"government & public":   {"government", "public", "citizen", "municipal", "tax", "voter", "civic", "federal", "state", "official"},
		"telecommunications":    {"telecom", "mobile", "broadband", "network", "wireless", "sim", "carrier", "phone", "data plan", "cellphone"},
		"gaming":                {"game", "gaming", "esport", "player", "multiplayer", "console", "steam", "gamer", "tournament", "play"},
		"automotive":            {"car", "vehicle", "auto", "motor", "drive", "dealer", "automobile", "truck", "electric vehicle", "fleet"},
	}

	lower := strings.ToLower(text)
	bestCategory := "unknown"
	bestMatches := 0
	bestTotal := 1

	for category, words := range keywords {
		matches := 0
		for _, word := range words {
			if strings.Contains(lower, word) {
				matches++
			}
		}
		if matches > bestMatches {
			bestCategory = category
			bestMatches = matches
			bestTotal = len(words)
		}
	}

	if bestMatches == 0 {
		return "unknown", 0
	}
	return bestCategory, roundFloat(float64(bestMatches) / float64(bestTotal) * 100)
}

// categorySimilarityScore compares two category strings.
func categorySimilarityScore(target, candidate string) float64 {
	if target == "" || candidate == "" ||
		target == "unknown" || candidate == "unknown" {
		return 0
	}
	if strings.EqualFold(target, candidate) {
		return 100
	}
	targetParts := strings.Fields(strings.ToLower(target))
	candidateParts := strings.Fields(strings.ToLower(candidate))
	if len(targetParts) > 0 && len(candidateParts) > 0 &&
		targetParts[0] == candidateParts[0] {
		return 60
	}
	return 0
}

// ─────────────────────────────────────────────────────────────────────────────
//  Threat text builders
// ─────────────────────────────────────────────────────────────────────────────

func buildThreatSummary(
	domainName string,
	threatType domain.ThreatType,
	score int,
	web domain.ThreatWebProfile,
	dns domain.ThreatDNSProfile,
) string {
	switch threatType {
	case domain.ThreatPhishing:
		return fmt.Sprintf("%s is phishing-capable with a live login workflow (score %d/100).", domainName, score)
	case domain.ThreatImpersonation:
		return fmt.Sprintf("%s shows brand-matching content — impersonation risk score %d/100.", domainName, score)
	default:
		switch {
		case dns.HasMX:
			return fmt.Sprintf("%s is a registered lookalike with active MX records (score %d/100).", domainName, score)
		case web.IsLive:
			return fmt.Sprintf("%s is a live lookalike domain (score %d/100).", domainName, score)
		default:
			return fmt.Sprintf("%s is a registered lookalike (score %d/100).", domainName, score)
		}
	}
}

func buildThreatDescription(
	project *domain.Project,
	candidate permutation,
	web domain.ThreatWebProfile,
	candidateCategory string,
	olderThanPrimary bool,
) string {
	parts := []string{
		fmt.Sprintf("Generated from %s via the %s permutation rule.", project.PrimaryDomain, candidate.Algorithm),
	}
	if web.IsLive {
		parts = append(parts, fmt.Sprintf("Serves HTTP %d content.", web.HTTPStatus))
	}
	if web.HasLoginForm {
		parts = append(parts, "A login or password flow is present.")
	}
	if candidateCategory != "unknown" {
		parts = append(parts, fmt.Sprintf("Page classified as: %s.", candidateCategory))
	}
	if olderThanPrimary {
		parts = append(parts, "Registration predates the monitored domain — reduces certainty.")
	}
	return strings.Join(parts, " ")
}

// ─────────────────────────────────────────────────────────────────────────────
//  Recommended actions
// ─────────────────────────────────────────────────────────────────────────────

func recommendedActions(
	domainName string,
	threatType domain.ThreatType,
	score int,
	hasMX, hasLoginForm, olderThanPrimary bool,
	registrar string,
) []domain.ThreatRecommendedAction {

	actions := []domain.ThreatRecommendedAction{
		{
			Key:         "registrar-abuse",
			Label:       "Registrar abuse report",
			Description: "File an abuse request with the registrar or via ICANN.",
			URL:         registrarAbuseURL(registrar),
			Priority:    1,
		},
	}

	if threatType == domain.ThreatPhishing || hasLoginForm {
		actions = append(actions,
			domain.ThreatRecommendedAction{
				Key:         "safe-browsing",
				Label:       "Report to Google Safe Browsing",
				Description: "Submit the domain for browser phishing protection.",
				URL:         "https://safebrowsing.google.com/safebrowsing/report_phish/",
				Priority:    2,
			},
			domain.ThreatRecommendedAction{
				Key:         "netcraft",
				Label:       "Submit to Netcraft",
				Description: "Use Netcraft's takedown flow for phishing domains.",
				URL:         "https://report.netcraft.com/report",
				Priority:    3,
			},
			domain.ThreatRecommendedAction{
				Key:         "microsoft-smartscreen",
				Label:       "Report to Microsoft SmartScreen",
				Description: "Flag the domain in Microsoft's browser protection system.",
				URL:         "https://www.microsoft.com/en-us/wdsi/support/report-unsafe-site",
				Priority:    4,
			},
		)
	}

	if score >= 70 && !olderThanPrimary {
		actions = append(actions, domain.ThreatRecommendedAction{
			Key:         "udrp",
			Label:       "Prepare UDRP complaint",
			Description: "Preserve evidence and prepare a domain dispute filing with ICANN.",
			URL:         "https://www.icann.org/resources/pages/help/dndr/udrp-en",
			Priority:    5,
		})
	}

	if score < 60 {
		actions = append(actions, domain.ThreatRecommendedAction{
			Key:         "defensive-registration",
			Label:       "Consider defensive registration",
			Description: "This variant is lower-risk but still worth defensive ownership.",
			URL:         "https://www.namecheap.com/domains/registration/results/?domain=" + domainName,
			Priority:    6,
		})
	}

	return actions
}

func registrarAbuseURL(registrar string) string {
	n := strings.ToLower(strings.TrimSpace(registrar))
	switch {
	case strings.Contains(n, "godaddy"):
		return "https://supportcenter.godaddy.com/AbuseReport"
	case strings.Contains(n, "namecheap"):
		return "https://support.namecheap.com/index.php?/Tickets/Submit"
	case strings.Contains(n, "porkbun"):
		return "https://porkbun.com/abuse"
	case strings.Contains(n, "tucows"):
		return "https://tucowsdomains.com/compliance-form/"
	case strings.Contains(n, "cloudflare"):
		return "https://www.cloudflare.com/abuse/form"
	case strings.Contains(n, "google"):
		return "https://domains.google/intl/en/abuse/"
	default:
		return "https://lookup.icann.org/en/lookup"
	}
}

func trustedRegistrar(registrar string) bool {
	n := strings.ToLower(strings.TrimSpace(registrar))
	return strings.Contains(n, "markmonitor") ||
		strings.Contains(n, "csc") ||
		strings.Contains(n, "corporate domains") ||
		strings.Contains(n, "brand shield") ||
		strings.Contains(n, "safenames")
}

// ─────────────────────────────────────────────────────────────────────────────
//  RDAP parsing helpers
// ─────────────────────────────────────────────────────────────────────────────

func extractRegistrar(payload map[string]any) string {
	entities, ok := payload["entities"].([]any)
	if !ok {
		return ""
	}
	for _, e := range entities {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if hasRole(m, "registrar") {
			if name := extractVCardText(m, "fn", "org"); name != "" {
				return name
			}
		}
	}
	for _, e := range entities {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if name := extractVCardText(m, "fn", "org"); name != "" {
			return name
		}
	}
	return ""
}

func extractRegistrationTime(payload map[string]any) *time.Time {
	events, ok := payload["events"].([]any)
	if !ok {
		return nil
	}
	for _, ev := range events {
		m, ok := ev.(map[string]any)
		if !ok {
			continue
		}
		action, _ := m["eventAction"].(string)
		dateStr, _ := m["eventDate"].(string)
		if action == "registration" || action == "registered" {
			if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
				utc := t.UTC()
				return &utc
			}
		}
	}
	return nil
}

func hasRole(entity map[string]any, target string) bool {
	roles, ok := entity["roles"].([]any)
	if !ok {
		return false
	}
	for _, r := range roles {
		if strings.EqualFold(fmt.Sprint(r), target) {
			return true
		}
	}
	return false
}

func extractVCardText(entity map[string]any, fieldNames ...string) string {
	vcard, ok := entity["vcardArray"].([]any)
	if !ok || len(vcard) < 2 {
		return ""
	}
	rows, ok := vcard[1].([]any)
	if !ok {
		return ""
	}
	for _, row := range rows {
		rs, ok := row.([]any)
		if !ok || len(rs) < 4 {
			continue
		}
		name := fmt.Sprint(rs[0])
		for _, wanted := range fieldNames {
			if strings.EqualFold(name, wanted) {
				return strings.TrimSpace(fmt.Sprint(rs[3]))
			}
		}
	}
	return ""
}

// ─────────────────────────────────────────────────────────────────────────────
//  Token / text helpers
// ─────────────────────────────────────────────────────────────────────────────

func brandTerms(project *domain.Project) []string {
	label, _ := splitDomain(project.PrimaryDomain)
	terms := []string{strings.ToLower(project.BrandName), strings.ToLower(label)}
	for _, kw := range project.MonitoringConfig.Keywords {
		terms = append(terms, strings.ToLower(kw))
	}
	return uniqueStrings(terms)
}

func tokenSet(text string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, t := range tokenize(text) {
		result[t] = struct{}{}
	}
	return result
}

func tokenize(text string) []string {
	cleaned := nonWordRe.ReplaceAllString(strings.ToLower(text), " ")
	cleaned = spaceRe.ReplaceAllString(cleaned, " ")
	parts := strings.Fields(cleaned)
	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) < 3 {
			continue
		}
		if _, skip := stopWords[p]; skip {
			continue
		}
		tokens = append(tokens, p)
	}
	return tokens
}

func cleanText(raw string) string {
	text := tagRe.ReplaceAllString(raw, " ")
	text = html.UnescapeString(text)
	text = spaceRe.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func extractTitle(raw string) string {
	m := titleRe.FindStringSubmatch(raw)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(cleanText(m[1]))
}

func containsAny(text string, terms []string) bool {
	for _, term := range terms {
		if n := strings.ToLower(strings.TrimSpace(term)); n != "" && strings.Contains(text, n) {
			return true
		}
	}
	return false
}

func jaccardScore(left, right map[string]struct{}) float64 {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	intersection := 0
	union := len(left)
	for t := range right {
		if _, ok := left[t]; ok {
			intersection++
		} else {
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return roundFloat(float64(intersection) / float64(union) * 100)
}

func stringSimilarity(left, right string) float64 {
	left = strings.ToLower(strings.TrimSpace(left))
	right = strings.ToLower(strings.TrimSpace(right))
	if left == "" || right == "" {
		return 0
	}
	if left == right {
		return 100
	}
	dist := levenshtein(left, right)
	maxLen := maxInt(len(left), len(right))
	if maxLen == 0 {
		return 100
	}
	return roundFloat((1 - float64(dist)/float64(maxLen)) * 100)
}

func levenshtein(left, right string) int {
	if len(left) == 0 {
		return len(right)
	}
	if len(right) == 0 {
		return len(left)
	}
	prev := make([]int, len(right)+1)
	curr := make([]int, len(right)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(left); i++ {
		curr[0] = i
		for j := 1; j <= len(right); j++ {
			cost := 1
			if left[i-1] == right[j-1] {
				cost = 0
			}
			curr[j] = minInt(minInt(curr[j-1]+1, prev[j]+1), prev[j-1]+cost)
		}
		copy(prev, curr)
	}
	return prev[len(right)]
}

// ─────────────────────────────────────────────────────────────────────────────
//  Threat lifecycle helpers
// ─────────────────────────────────────────────────────────────────────────────

func severityFromScore(score int) string {
	switch {
	case score >= 90:
		return "critical"
	case score >= 75:
		return "high"
	case score >= 45:
		return "medium"
	default:
		return "low"
	}
}

func detectedAt(registrationDate *time.Time) time.Time {
	if registrationDate == nil {
		return time.Now().UTC()
	}
	return registrationDate.UTC()
}

// ─────────────────────────────────────────────────────────────────────────────
//  Misc helpers
// ─────────────────────────────────────────────────────────────────────────────

func dnsLegacyMap(profile domain.ThreatDNSProfile) map[string]string {
	records := map[string]string{}
	if len(profile.ARecords) > 0 {
		records["A"] = strings.Join(profile.ARecords, ", ")
	}
	if len(profile.MXRecords) > 0 {
		records["MX"] = strings.Join(profile.MXRecords, ", ")
	}
	if len(profile.NSRecords) > 0 {
		records["NS"] = strings.Join(profile.NSRecords, ", ")
	}
	if len(profile.TXTRecords) > 0 {
		records["TXT"] = strings.Join(profile.TXTRecords, ", ")
	}
	return records
}

func sortThreats(findings []domain.Threat) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Score != findings[j].Score {
			return findings[i].Score > findings[j].Score
		}
		// Tie-break by SimilarityScore (higher = closer to brand domain).
		// This ensures close mutations (edit distance 1-2) rank above
		// prefix/suffix variants when both have the same threat score.
		if findings[i].Details.SimilarityScore != findings[j].Details.SimilarityScore {
			return findings[i].Details.SimilarityScore > findings[j].Details.SimilarityScore
		}
		return findings[i].DetectedAt.After(findings[j].DetectedAt)
	})
}

func cloneThreats(findings []domain.Threat) []domain.Threat {
	cloned := make([]domain.Threat, len(findings))
	copy(cloned, findings)
	return cloned
}

func extractHTTPBanner(header http.Header) string {
	var parts []string
	if s := strings.TrimSpace(header.Get("Server")); s != "" {
		parts = append(parts, "Server: "+s)
	}
	if p := strings.TrimSpace(header.Get("X-Powered-By")); p != "" {
		parts = append(parts, "X-Powered-By: "+p)
	}
	return strings.Join(parts, " | ")
}

func detectTechnologies(header http.Header, lowerBody string) []string {
	combined := strings.ToLower(header.Get("Server")) + " " +
		strings.ToLower(header.Get("X-Powered-By")) + " " +
		lowerBody

	sigs := []struct{ needle, label string }{
		{"cloudflare", "Cloudflare"}, {"nginx", "Nginx"}, {"apache", "Apache"},
		{"openresty", "OpenResty"}, {"litespeed", "LiteSpeed"}, {"iis", "IIS"},
		{"php", "PHP"}, {"express", "Express"}, {"__next_data__", "Next.js"},
		{"_next/static", "Next.js"}, {"react", "React"}, {"ng-version", "Angular"},
		{"vue", "Vue"}, {"jquery", "jQuery"}, {"bootstrap", "Bootstrap"},
		{"tailwind", "Tailwind CSS"}, {"wp-content", "WordPress"},
		{"woocommerce", "WooCommerce"}, {"shopify", "Shopify"},
		{"squarespace", "Squarespace"}, {"wix", "Wix"}, {"asp.net", "ASP.NET"},
	}
	techs := make([]string, 0, 4)
	for _, sig := range sigs {
		if strings.Contains(combined, sig.needle) {
			techs = append(techs, sig.label)
		}
	}
	return uniqueStrings(techs)
}

func appendIfMissing(values []string, value string) []string {
	for _, v := range values {
		if v == value {
			return values
		}
	}
	return append(values, value)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func brandBoost(looksLikeBrand bool) float64 {
	if looksLikeBrand {
		return 12
	}
	return 0
}

func roundFloat(v float64) float64 { return math.Round(v*10) / 10 }

func insecureTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
