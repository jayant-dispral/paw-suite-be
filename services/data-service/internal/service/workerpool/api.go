package workerpool

import (
	"fmt"
	"math/rand"
	"time"


	"github.com/jayant-dispral/brand-threat-be/shared/domain"
)

func (wp *WorkerPool) callScrapingAPI(task Task) (Result, error) {
    // Simulate API latency (2-6 seconds)
    latency := time.Duration(2000+rand.Intn(4000)) * time.Millisecond
    time.Sleep(latency)
    
    // Simulate occasional errors (5% failure rate)
    if rand.Float32() < 0.05 {
        return Result{}, fmt.Errorf("API error: rate limited (429)")
    }
    
    // Generate mock social posts (10-20 posts)
    numPosts := 10 + rand.Intn(11)
    posts := make([]domain.SocialPost, numPosts)
    
    now := time.Now()
    fetchedAt := now
    
    for i := 0; i < numPosts; i++ {
        externalID := fmt.Sprintf("twitter:%d%d", now.Unix(), rand.Intn(1000000))
        
        posts[i] = domain.SocialPost{
            ProjectID:  task.ProjectID,
            Platform:   "twitter",
            ExternalID: externalID,
            Content:    fmt.Sprintf("Mock tweet about %v - #%d", task.Keywords, i+1),
            URL:        fmt.Sprintf("https://twitter.com/user/status/%s", externalID),
            PostedAt:   now.Add(-time.Duration(rand.Intn(3600)) * time.Second),
            
            Author: domain.Author{
                ID:        fmt.Sprintf("user_%d", rand.Intn(1000000)),
                Username:  fmt.Sprintf("user_%d", rand.Intn(10000)),
                Name:      fmt.Sprintf("Mock User %d", i+1),
                Verified:  rand.Float32() < 0.1, // 10% verified
                Followers: rand.Intn(10000),
            },
            
            Engagement: domain.Engagement{
                Likes:    rand.Intn(500),
                Shares:   rand.Intn(100),
                Comments: rand.Intn(50),
                Views:    rand.Intn(5000),
            },
            
            Sentiment:      domain.SentimentNeutral,
            SentimentScore: -1.0, // Not calculated yet
            
            Entities: domain.Entities{
                Hashtags: task.Keywords,
                URLs:     []string{},
                Mentions: []string{},
            },
            
            MatchedKeywords: task.Keywords,
            IsViral:         false,
            
            FetchedAt:  fetchedAt,
            IngestedAt: now,
            ExpiresAt:  now.AddDate(0, 0, 7), // 7 days retention
            CreatedAt:  now,
            UpdatedAt:  now,
        }
    }
    
    return Result{
        TaskID:      task.ID,
        SocialPosts: posts,
        FetchedAt:   fetchedAt,
    }, nil
}