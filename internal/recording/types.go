package recording

import (
	"encoding/json"
	"time"
)

type Record struct {
	Timestamp Timestamp `json:"ts"`
	Position  []float32 `json:"pos"`
	Rotation  []float32 `json:"rot"`
}

// ISO 8601 format with nanosecond precision
const TimeFormat = "2006-01-02T15:04:05.999999999Z"

type Timestamp struct {
	Time time.Time
}

func (t *Timestamp) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Time.UTC().Format(TimeFormat))
}

func (t *Timestamp) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	time, err := time.Parse(TimeFormat, s)
	if err != nil {
		return err
	}
	t.Time = time
	return nil
}
