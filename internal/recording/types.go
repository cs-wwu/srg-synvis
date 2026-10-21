package recording

import (
	"encoding/json"
	"errors"
	"time"
)

type Record struct {
	Timestamp *Timestamp
	Event     Event
}

type Event interface {
	Name() string
}

func EventFromName(name string) Event {
	switch name {
	case "started":
		return &EventStarted{}
	case "stopped":
		return &EventStopped{}
	case "gps":
		return &EventGpsPoint{}
	default:
		return nil
	}
}

type EventStarted struct{}

func (e *EventStarted) Name() string {
	return "started"
}

type EventStopped struct{}

func (e *EventStopped) Name() string {
	return "stopped"
}

type EventGpsPoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
	Speed     float64 `json:"speed"`
}

func (e *EventGpsPoint) Name() string {
	return "gps"
}

type EventHatOrientation struct {
	Pitch float64 `json:"pitch"`
	Roll  float64 `json:"roll"`
	Yaw   float64 `json:"yaw"`
}

func (e *EventHatOrientation) Name() string {
	return "hat"
}

func (r *Record) MarshalJSON() ([]byte, error) {
	// NB: Different from in UnmarshalJSON, because we know the type of r.Event here.
	type RecordData struct {
		Timestamp *Timestamp `json:"ts"`
		Event     string     `json:"event"`
		Data      Event      `json:"data"`
	}

	return json.Marshal(RecordData{
		Timestamp: r.Timestamp,
		Event:     r.Event.Name(),
		Data:      r.Event,
	})
}

func (r *Record) UnmarshalJSON(data []byte) error {
	// Type of `Data` is unknown here, so we use json.RawMessage.
	type RecordData struct {
		Timestamp *Timestamp      `json:"ts"`
		Event     string          `json:"event"`
		Data      json.RawMessage `json:"data"`
	}

	var aux RecordData
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	r.Timestamp = aux.Timestamp
	event := EventFromName(aux.Event)
	if event == nil {
		return errors.New("unknown event type: " + aux.Event)
	}
	if err := json.Unmarshal(aux.Data, &event); err != nil {
		return err
	}
	r.Event = event
	return nil
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
