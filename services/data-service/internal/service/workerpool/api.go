package workerpool

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	scraper "github.com/jayant-dispral/brand-threat-be/services/data-service/pkg/scrapper"
	"github.com/jayant-dispral/brand-threat-be/shared/domain"
)

func (wp *WorkerPool) callScrapingAPI(task Task) (Result, error) {
	query := strings.Join(task.Keywords, " OR ")

	url := fmt.Sprintf(
		"https://twitter241.p.rapidapi.com/search-v3?type=Top&count=30&query=%s",
		url.QueryEscape(query),
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Result{}, err
	}

	req.Header.Set("x-rapidapi-key", os.Getenv("RAPIDAPI_KEY"))
	req.Header.Set("x-rapidapi-host", "twitter241.p.rapidapi.com")

	start := time.Now()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Result{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("twitter api error: %s", resp.Status)
	}

	var parsed scraper.TwitterSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return Result{}, err
	}

	now := time.Now()

	posts := make([]domain.SocialPost, 0, 30)

	for _, instr := range parsed.Result.TimelineResponse.Timeline.Instructions {
		for _, entry := range instr.Entries {
			if !strings.HasPrefix(entry.EntryID, "tweet-") {
				continue
			}

			tweet := entry.Content.Content.TweetResults.Result
			// Critical field checks
			if tweet.RestID == "" ||
				tweet.Core.UserResults.Result.RestID == "" {
				continue
			}

			text := tweet.Legacy.FullText
			var hashtags []string
			var urls []string

			if tweet.NoteTweet != nil {
				text = tweet.NoteTweet.NoteTweetResults.Result.Text

				for _, h := range tweet.NoteTweet.NoteTweetResults.Result.EntitySet.Hashtags {
					hashtags = append(hashtags, h.Text)
				}

				for _, u := range tweet.NoteTweet.NoteTweetResults.Result.EntitySet.Urls {
					urls = append(urls, u.ExpandedURL)
				}
			}

			author := tweet.Core.UserResults.Result

			post := domain.SocialPost{
				ProjectID:  task.ProjectID,
				Platform:   "twitter",
				ExternalID: "twitter:" + tweet.RestID,
				Content:    text,
				URL: fmt.Sprintf(
					"https://twitter.com/%s/status/%s",
					author.Legacy.ScreenName,
					tweet.RestID,
				),
				PostedAt: time.UnixMilli(tweet.Legacy.CreatedAtMs),

				Author: domain.Author{
					ID:        author.RestID,
					Username:  author.Legacy.ScreenName,
					Name:      author.Legacy.Name,
					Verified:  author.Legacy.Verified,
					Followers: author.Legacy.Followers,
				},

				Engagement: domain.Engagement{
					Likes:    tweet.Legacy.Counts.FavoriteCount,
					Shares:   tweet.Legacy.Counts.RetweetCount,
					Comments: tweet.Legacy.Counts.ReplyCount,
				},

				Sentiment:      domain.SentimentNeutral,
				SentimentScore: -1,
				Entities: domain.Entities{
					Hashtags: hashtags,
					URLs:     urls,
				},
				MatchedKeywords: task.Keywords,

				FetchedAt:  start,
				IngestedAt: now,
				ExpiresAt:  now.AddDate(0, 0, 7),
				CreatedAt:  now,
				UpdatedAt:  now,
			}
			posts = append(posts, post)
		}
	}

	return Result{
		TaskID:      task.ID,
		SocialPosts: posts,
		FetchedAt:   start,
	}, nil

}
