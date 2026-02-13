package scrapper

type TwitterSearchResponse struct {
	Status     string         `json:"status"`
	Timeline   []TwitterTweet `json:"timeline"`
	NextCursor string         `json:"next_cursor"`
	PrevCursor string         `json:"prev_cursor"`
}

type TwitterTweet struct {
	Type           string `json:"type"`
	TweetID        string `json:"tweet_id"`
	ScreenName     string `json:"screen_name"`
	Bookmarks      int    `json:"bookmarks"`
	Favorites      int    `json:"favorites"`
	CreatedAt      string `json:"created_at"`
	Text           string `json:"text"`
	Lang           string `json:"lang"`
	Quotes         int    `json:"quotes"`
	Replies        int    `json:"replies"`
	Retweets       int    `json:"retweets"`
	Views          any    `json:"views"` // can be string or null
	ConversationID string `json:"conversation_id"`

	Entities struct {
		Hashtags []struct {
			Text string `json:"text"`
		} `json:"hashtags"`

		Urls []struct {
			ExpandedURL string `json:"expanded_url"`
		} `json:"urls"`

		UserMentions []struct {
			ScreenName string `json:"screen_name"`
		} `json:"user_mentions"`
	} `json:"entities"`

	UserInfo struct {
		RestID         string `json:"rest_id"`
		ScreenName     string `json:"screen_name"`
		Name           string `json:"name"`
		FollowersCount int    `json:"followers_count"`
		Verified       bool   `json:"verified"`
	} `json:"user_info"`
}
