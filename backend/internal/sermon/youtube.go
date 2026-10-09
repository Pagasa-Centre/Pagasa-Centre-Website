package sermon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"pagasacentre/backend/internal/sermon/domain"
)

const youtubeSearchURL = "https://www.googleapis.com/youtube/v3/search"

type YouTubeClient struct {
	apiKey    string
	channelID string
	http      *http.Client
}

func NewYouTubeClient(apiKey, channelID string) *YouTubeClient {
	return &YouTubeClient{
		apiKey:    apiKey,
		channelID: channelID,
		http:      &http.Client{Timeout: 15 * time.Second},
	}
}

type youtubeSearchResponse struct {
	Items []struct {
		ID struct {
			VideoID string `json:"videoId"`
		} `json:"id"`
		Snippet struct {
			Title       string `json:"title"`
			PublishedAt string `json:"publishedAt"`
		} `json:"snippet"`
	} `json:"items"`
}

func (c *YouTubeClient) LatestCompleted(ctx context.Context) (*domain.Sermon, error) {
	q := url.Values{}
	q.Set("part", "snippet")
	q.Set("channelId", c.channelID)
	q.Set("eventType", "completed")
	q.Set("type", "video")
	q.Set("order", "date")
	q.Set("maxResults", "1")
	q.Set("key", c.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, youtubeSearchURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("youtube request: %w", err)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("youtube http: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube api status %d", res.StatusCode)
	}

	var body youtubeSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("youtube decode: %w", err)
	}
	if len(body.Items) == 0 {
		return nil, nil
	}

	item := body.Items[0]
	if item.ID.VideoID == "" {
		return nil, nil
	}

	var publishedAt time.Time
	if item.Snippet.PublishedAt != "" {
		if t, err := time.Parse(time.RFC3339, item.Snippet.PublishedAt); err == nil {
			publishedAt = t
		}
	}

	return &domain.Sermon{
		VideoID:     item.ID.VideoID,
		Title:       item.Snippet.Title,
		PublishedAt: publishedAt,
	}, nil
}
