package pkg

import (
	"context"

	"golang.org/x/time/rate"
)

type Task struct {
	TaskID  string
	Payload []byte
}

type WorkerPool struct {

	//configurations
	apiRateLimit       int
	apiWorkerCount     int
	processWorkerCount int

	//channels
	dispatchQueue   chan Task
	apiWorkQueue    chan Task
	processingQueue chan Task

	//rate limiter
	rateLimiter *rate.Limiter

	//context for shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

type Result struct {
	TaskID string
	data   []byte
	//scraped data
}

func NewWorkerPool(apiRate, apiWorkers, processWorkers int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		apiRateLimit:       apiRate,
		apiWorkerCount:     apiWorkers,
		processWorkerCount: processWorkers,
		dispatchQueue:      make(chan Task, 50),
		apiWorkQueue:       make(chan Task, 50),
		processingQueue:    make(chan Task, 20),
		rateLimiter:        rate.NewLimiter(rate.Limit(apiRate), apiRate),
		ctx:                ctx,
		cancel:             cancel,
	}
}
