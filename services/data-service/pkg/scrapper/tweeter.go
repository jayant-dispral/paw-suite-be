package scrapper
type TwitterSearchResponse struct {
	Cursor struct {
		Bottom string `json:"bottom"`
	} `json:"cursor"`

	Result struct {
		TimelineResponse struct {
			Timeline struct {
				Instructions []struct {
					Entries []struct {
						EntryID string `json:"entry_id"`
						Content struct {
							Content struct {
								TweetResults struct {
									Result twitterTweet `json:"result"`
								} `json:"tweet_results"`
							} `json:"content"`
						} `json:"content"`
					} `json:"entries"`
				} `json:"instructions"`
			} `json:"timeline"`
		} `json:"timeline_response"`
	} `json:"result"`
}

type twitterTweet struct {
	RestID string `json:"rest_id"`

	Legacy struct {
		CreatedAtMs int64  `json:"created_at_ms"`
		FullText    string `json:"full_text"`
		Counts      struct {
			FavoriteCount int `json:"favorite_count"`
			RetweetCount  int `json:"retweet_count"`
			ReplyCount    int `json:"reply_count"`
		} `json:"counts"`
	} `json:"legacy"`

	NoteTweet *struct {
		NoteTweetResults struct {
			Result struct {
				Text string `json:"text"`
				EntitySet struct {
					Hashtags []struct {
						Text string `json:"text"`
					} `json:"hashtags"`
					Urls []struct {
						ExpandedURL string `json:"expanded_url"`
					} `json:"urls"`
				} `json:"entity_set"`
			} `json:"result"`
		} `json:"note_tweet_results"`
	} `json:"note_tweet"`

	Core struct {
		UserResults struct {
			Result struct {
				RestID string `json:"rest_id"`
				Legacy struct {
					ScreenName    string `json:"screen_name"`
					Name          string `json:"name"`
					Verified      bool   `json:"verified"`
					Followers     int    `json:"followers_count"`
				} `json:"legacy"`
			} `json:"result"`
		} `json:"user_results"`
	} `json:"core"`
}
