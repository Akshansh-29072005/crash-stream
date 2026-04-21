package tasks

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

// Task names
const (
	TypeAccidentDetection = "accident:detection"
)

// DetectionTaskPayload represents the payload for an accident detection task
type DetectionTaskPayload struct {
	ImageData []byte `json:"image_data"`
	Timestamp string `json:"timestamp"`
}

// NewDetectionTask creates a new asynq task for accident detection
func NewDetectionTask(imageData []byte, timestamp string) (*asynq.Task, error) {
	payload, err := json.Marshal(DetectionTaskPayload{
		ImageData: imageData,
		Timestamp: timestamp,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeAccidentDetection, payload), nil
}
