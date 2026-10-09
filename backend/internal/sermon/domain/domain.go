package domain

import "time"

type Sermon struct {
	VideoID     string
	Title       string
	PublishedAt time.Time
}
