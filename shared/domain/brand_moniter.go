package domain

import "time"

type BrandMonitorEvent struct {
	ProjectID   string    `json:"project_id"`
	KeyWord     string    `json:"key_word"`
	RequestedBy string    `json:"requested_by"`
	TimeStamp   time.Time `json:"time_stamp"`
}
