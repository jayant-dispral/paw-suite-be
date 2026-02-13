package workerpool

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	scrapper "github.com/jayant-dispral/brand-threat-be/services/data-service/pkg/scrapper"
	"github.com/jayant-dispral/brand-threat-be/shared/domain"
)

func (wp *WorkerPool) callScrapingAPI(task Task) (*Result, error) {
	query := strings.Join(task.Keywords, " OR ")

	apiURL := fmt.Sprintf(
		"https://twitter-api45.p.rapidapi.com/search.php?query=%s&search_type=Latest",
		url.QueryEscape(query),
	)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("x-rapidapi-key", os.Getenv("RAPIDAPI_KEY"))
	req.Header.Set("x-rapidapi-host", os.Getenv("RAPIDAPI_HOST"))

	start := time.Now()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("twitter api error: %s", resp.Status)
	}

	var parsed scrapper.TwitterSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	if parsed.Status != "ok" {
		return nil, fmt.Errorf("provider returned status: %s", parsed.Status)
	}

	now := time.Now()
	posts := make([]domain.SocialPost, 0, len(parsed.Timeline))

	for _, tweet := range parsed.Timeline {

		if tweet.Type != "tweet" {
			continue
		}

		if tweet.TweetID == "" || tweet.UserInfo.RestID == "" {
			continue
		}

		// Parse time
		postedAt, err := time.Parse(time.RubyDate, tweet.CreatedAt)
		if err != nil {
			continue
		}

		// Parse views safely
		views := 0
		switch v := tweet.Views.(type) {
		case string:
			if parsedViews, err := strconv.Atoi(v); err == nil {
				views = parsedViews
			}
		case float64:
			views = int(v)
		}

		// Extract entities
		var hashtags []string
		for _, h := range tweet.Entities.Hashtags {
			hashtags = append(hashtags, h.Text)
		}

		var urlsList []string
		for _, u := range tweet.Entities.Urls {
			urlsList = append(urlsList, u.ExpandedURL)
		}

		var mentions []string
		for _, m := range tweet.Entities.UserMentions {
			mentions = append(mentions, m.ScreenName)
		}

		post := domain.SocialPost{
			ProjectID:  task.ProjectID,
			Platform:   "twitter",
			ExternalID: "twitter:" + tweet.TweetID,
			Content:    tweet.Text,
			URL: fmt.Sprintf(
				"https://twitter.com/%s/status/%s",
				tweet.ScreenName,
				tweet.TweetID,
			),
			PostedAt: postedAt,

			Author: domain.Author{
				ID:        tweet.UserInfo.RestID,
				Username:  tweet.UserInfo.ScreenName,
				Name:      tweet.UserInfo.Name,
				Verified:  tweet.UserInfo.Verified,
				Followers: tweet.UserInfo.FollowersCount,
			},

			Engagement: domain.Engagement{
				Likes:    tweet.Favorites,
				Shares:   tweet.Retweets,
				Comments: tweet.Replies,
				Views:    views,
			},

			Sentiment:      domain.SentimentNeutral,
			SentimentScore: -1,
			Entities: domain.Entities{
				Hashtags: hashtags,
				URLs:     urlsList,
				Mentions: mentions,
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

	return &Result{
		TaskID:      task.ID,
		SocialPosts: posts,
		FetchedAt:   start,
	}, nil
}
