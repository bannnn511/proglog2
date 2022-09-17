package log

import (
	"bytes"
	"os"
	api "proglog/api/v1"
	"testing"
)

func setupLog() (*Log, error) {
	dir, _ := os.MkdirTemp("", "log-test")

	c := &Config{}
	c.Segment.MaxStoreBytes = 32
	log, err := NewLog(dir, *c)
	if err != nil {
		return nil, err
	}

	return log, nil
}

func TestLog_Append_Read(t *testing.T) {
	log, err := setupLog()
	defer log.Remove()

	if err != nil {
		t.Errorf("setupLog() error ")
	}
	value := &api.Record{
		Value: []byte("value"),
	}

	off, err := log.Append(value)
	if err != nil {
		t.Errorf("Append() error %v", err)
		return
	}

	read, err := log.Read(off)
	if err != nil {
		t.Errorf("Read() error %v", err)
		return
	}

	if bytes.Compare(read.Value, value.Value) != 0 {
		t.Errorf("want %v, got %v", value, read)
		return
	}
}

func TestLog_Out_of_Range(t *testing.T) {
	log, err := setupLog()
	defer log.Remove()
	if err != nil {
		t.Errorf("setupLog() error ")
	}

	_, err = log.Read(1)
	if err != err.(api.ErrorOffsetOfOutRange) {
		t.Errorf("expected %v, got %v", api.ErrorOffsetOfOutRange{}, err)
	}
}

func TestLogExists(t *testing.T) {
	log, err := setupLog()
	defer log.Remove()

	if err != nil {
		t.Errorf("setupLog() error ")
	}
	value := &api.Record{
		Value: []byte("value"),
	}

	for i := 0; i < 3; i++ {
		_, err := log.Append(value)
		if err != nil {
			t.Errorf("Append() error %v", err)
			return
		}
	}

	off, err := log.LowestOffset()
	if err != nil {
		t.Errorf("LowestOffset() error %v", err)
	}
	if off != 0 {
		t.Errorf("got %v, want %v", off, 0)
	}

	off, err = log.HighestOffset()
	if err != nil {
		t.Errorf("HighestOffset() error %v", err)
	}
	if off != 2 {
		t.Errorf("got %v, want %v", off, 2)
	}

	log2, err := NewLog(log.dir, log.config)
	off, err = log2.LowestOffset()
	if err != nil {
		t.Errorf("LowestOffset() error %v", err)
	}
	if off != 0 {
		t.Errorf("got %v, want %v", off, 0)
	}

	off, err = log2.HighestOffset()
	if err != nil {
		t.Errorf("HighestOffset() error %v", err)
	}
	if off != 2 {
		t.Errorf("got %v, want %v", off, 2)
	}
}

func TestLog_Truncate(t *testing.T) {
	log, err := setupLog()
	defer log.Remove()

	if err != nil {
		t.Errorf("setupLog() error ")
	}
	value := &api.Record{
		Value: []byte("value"),
	}

	for i := 0; i < 3; i++ {
		_, err := log.Append(value)
		if err != nil {
			t.Errorf("Append() error %v", err)
			return
		}
	}

	err = log.Truncate(1)
	if err != nil {
		t.Errorf("Truncate() error %v", err)
	}

	_, err = log.Read(0)
	if err == nil {
		t.Errorf("expected error")
	}
}
