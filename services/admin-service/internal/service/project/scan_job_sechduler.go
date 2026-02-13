package project

import (
	"context"
	"log"
	"time"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	"github.com/jayant-dispral/brand-threat-be/shared/ports"
)

type BrandEventScanner struct {
	projectRepo  ports.ProjectRepository
	publisher    ports.EventPublisher
	tickInterval time.Duration
	batchSize    int
}

func NewBrandEventScanner(
	repo ports.ProjectRepository,
	publisher ports.EventPublisher,
	tick time.Duration,
	batch int,
) *BrandEventScanner {
	return &BrandEventScanner{
		projectRepo:  repo,
		publisher:    publisher,
		tickInterval: tick,
		batchSize:    batch,
	}
}

// es stands for event scanner
func (es *BrandEventScanner) Start(ctx context.Context) error {
	log.Printf("[Scheduler] started (tick=%v)", es.tickInterval)

	ticker := time.NewTicker(es.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			es.runTick(ctx)
		case <-ctx.Done():
			log.Printf("[Scheduler] shutting down")
			return ctx.Err()
		}
	}
}

func (es *BrandEventScanner) runTick(ctx context.Context) {
	now := time.Now()
	log.Println("[Sechduler]: started db scan at: ", now.Local())
	projects, err := es.projectRepo.ClaimDueProjects(
		ctx,
		now,
		es.batchSize,
	)
	if err != nil {
		log.Printf("[Scheduler] claim error: %v", err)
		return
	}
	log.Printf("[Sechduler]: Found: %v projects in current scan at: %v ", len(projects), now.Local())
	for _, project := range projects {

		event := domain.BrandMonitorEvent{
			ProjectID: project.ID.Hex(),
			KeyWords:  project.MonitoringConfig.Keywords,
			Timestamp: now,
		}

		if err := es.publisher.PublishBrandMonitorEvent(ctx, &event); err != nil {
			log.Printf("[Scheduler] publish failed: %v", err)
			continue
		}

		log.Printf(
			"[Scheduler] ✓ emitted scan job project=%s",
			project.ID.Hex(),
		)
	}
}
