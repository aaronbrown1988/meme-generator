package db

import "time"

type Generation struct {
	ID           int64     `json:"id"`
	Prompt       string    `json:"prompt"`
	ImagePath    string    `json:"image_path"`
	TopText      string    `json:"top_text"`
	BottomText   string    `json:"bottom_text"`
	Status       string    `json:"status"`
	ErrorMessage *string   `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

const (
	StatusProcessing = "processing"
	StatusSuccess    = "success"
	StatusFailed     = "failed"
)
