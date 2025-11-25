package recording

import (
	"bufio"
	"encoding/json"
	"os"
	"time"
)

const FileName = "recording.json"

type PositionLogger struct {
	file    *os.File
	writer  *bufio.Writer
	encoder *json.Encoder
}

func NewLogger() (*PositionLogger, error) {
	file, err := os.OpenFile(FileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	writer := bufio.NewWriter(file)
	return &PositionLogger{
		file:    file,
		writer:  writer,
		encoder: json.NewEncoder(writer),
	}, nil
}

func (pl *PositionLogger) Close() error {
	if err := pl.writer.Flush(); err != nil {
		return err
	}
	return pl.file.Close()
}

func (pl *PositionLogger) Log(event Event) error {
	record := Record{
		Timestamp: &Timestamp{time.Now()},
		Event:     event,
	}

	if err := pl.encoder.Encode(&record); err != nil {
		return err
	}
	return pl.writer.Flush()
}
