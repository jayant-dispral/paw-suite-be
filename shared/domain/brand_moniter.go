package domain

import "time"

type BrandMonitorEvent struct {
	ProjectID   string    `json:"project_id"`
	KeyWords    []string  `json:"key_words"`
	RequestedBy string    `json:"requested_by"`
	Timestamp   time.Time `json:"time_stamp"`
}
