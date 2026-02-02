// internal/services/workerpool/workerpool.go
package workerpool

import (
    "context"
    "fmt"
    "time"
    
    "golang.org/x/time/rate"
    "go.mongodb.org/mongo-driver/mongo"
)

type WorkerPool struct {
    // Configuration
    apiRateLimit    int
    apiWorkerCount  int
    procWorkerCount int
    
    // Channels
    dispatchQueue   chan Task
    apiWorkQueue    chan Task
    processingQueue chan Result
    
    // Rate limiter
    rateLimiter *rate.Limiter
    
    // Database
    db *mongo.Database
    
    // Context for shutdown
    ctx    context.Context
    cancel context.CancelFunc
}

func NewWorkerPool(apiRate, apiWorkers, procWorkers int, db *mongo.Database) *WorkerPool {
    ctx, cancel := context.WithCancel(context.Background())
    
    return &WorkerPool{
        apiRateLimit:    apiRate,
        apiWorkerCount:  apiWorkers,
        procWorkerCount: procWorkers,
        
        dispatchQueue:   make(chan Task, 50),
        apiWorkQueue:    make(chan Task, 50),
        processingQueue: make(chan Result, 20),
        
        rateLimiter: rate.NewLimiter(rate.Limit(apiRate), apiRate),
        
        db: db,
        
        ctx:    ctx,
        cancel: cancel,
    }
}

func (wp *WorkerPool) Start() {
    fmt.Println("Starting worker pool...")
    
    // Start dispatcher
    go wp.dispatcher()
    
    // Start API workers
    for i := 0; i < wp.apiWorkerCount; i++ {
        go wp.apiWorker(i)
    }
    
    // Start processing workers
    for i := 0; i < wp.procWorkerCount; i++ {
        go wp.processingWorker(i)
    }
    
    fmt.Printf("Worker pool started: %d API workers, %d processing workers\n",
        wp.apiWorkerCount, wp.procWorkerCount)
}

func (wp *WorkerPool) dispatcher() {
    fmt.Println("Dispatcher started")
    
    for {
        select {
        case task := <-wp.dispatchQueue:
            // Wait for rate limiter token
            if err := wp.rateLimiter.Wait(wp.ctx); err != nil {
                fmt.Println("Dispatcher shutting down:", err)
                return
            }
            
            // Send to API worker queue
            select {
            case wp.apiWorkQueue <- task:
                // Successfully queued
            case <-wp.ctx.Done():
                fmt.Println("Dispatcher shutting down while queuing")
                return
            }
            
        case <-wp.ctx.Done():
            fmt.Println("Dispatcher shutting down")
            return
        }
    }
}

func (wp *WorkerPool) apiWorker(id int) {
    fmt.Printf("API Worker %d started\n", id)
    
    for {
        select {
        case task := <-wp.apiWorkQueue:
            startTime := time.Now()
            fmt.Printf("[API Worker %d] Processing task %s (keywords: %v)\n",
                id, task.ID, task.Keywords)
            
            // Call external scraping API (mock for now)
            result, err := wp.callScrapingAPI(task)
            if err != nil {
                fmt.Printf("[API Worker %d] ERROR: %v\n", id, err)
                
                // Check if shutdown during error
                select {
                case <-wp.ctx.Done():
                    return
                default:
                    continue
                }
            }
            
            duration := time.Since(startTime)
            fmt.Printf("[API Worker %d] Completed task %s in %v, found %d posts\n",
                id, task.ID, duration, len(result.SocialPosts))
            
            // Send to processing queue
            select {
            case wp.processingQueue <- result:
                // Queued successfully
            case <-wp.ctx.Done():
                fmt.Printf("[API Worker %d] Shutdown, dropping result for task %s\n",
                    id, task.ID)
                return
            }
            
            // Check shutdown after completing task
            select {
            case <-wp.ctx.Done():
                fmt.Printf("[API Worker %d] Graceful shutdown\n", id)
                return
            default:
                // Continue
            }
            
        case <-wp.ctx.Done():
            fmt.Printf("[API Worker %d] Shutdown while idle\n", id)
            return
        }
    }
}

func (wp *WorkerPool) processingWorker(id int) {
    fmt.Printf("Processing Worker %d started\n", id)
    
    for {
        select {
        case result := <-wp.processingQueue:
            fmt.Printf("[Processing Worker %d] Processing %d posts from task %s\n",
                id, len(result.SocialPosts), result.TaskID)
            
            // Run analytics (pass-through for now)
            if err := wp.runAnalytics(&result); err != nil {
                fmt.Printf("[Processing Worker %d] Analytics failed: %v\n", id, err)
                continue
            }
            
            // Save to database (one by one with upsert)
            successCount := 0
            for _, post := range result.SocialPosts {
                if err := wp.saveToDB(post); err != nil {
                    fmt.Printf("[Processing Worker %d] DB save failed for post %s: %v\n",
                        id, post.ExternalID, err)
                    // Continue with next post (don't fail entire batch)
                    continue
                }
                successCount++
            }
            
            fmt.Printf("[Processing Worker %d] Saved %d/%d posts from task %s\n",
                id, successCount, len(result.SocialPosts), result.TaskID)
            
            // Check shutdown after completing task
            select {
            case <-wp.ctx.Done():
                fmt.Printf("[Processing Worker %d] Graceful shutdown\n", id)
                return
            default:
                // Continue
            }
            
        case <-wp.ctx.Done():
            fmt.Printf("[Processing Worker %d] Shutdown while idle\n", id)
            return
        }
    }
}

// SubmitTask adds a task to the worker pool
func (wp *WorkerPool) SubmitTask(task Task) error {
    select {
    case wp.dispatchQueue <- task:
        return nil
    case <-wp.ctx.Done():
        return fmt.Errorf("worker pool is shutting down")
    default:
        return fmt.Errorf("dispatch queue is full")
    }
}

func (wp *WorkerPool) Shutdown(timeout time.Duration) error {
    fmt.Println("Starting graceful shutdown...")
    
    // Signal all workers to stop
    wp.cancel()
    
    // In production, use WaitGroup here
    // For now, just wait for timeout
    time.Sleep(timeout)
    
    fmt.Println("Shutdown complete")
    return nil
}