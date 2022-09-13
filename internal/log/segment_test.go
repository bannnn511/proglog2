package log

import (
	"bytes"
	"io"
	"os"
	api "proglog/api/v1"
	"testing"
)

func TestSegment(t *testing.T) {
	dir, _ := os.MkdirTemp("", "test-segment")
	config := config
	config.MaxIndexBytes = entryWidth * 2
	config.MaxStoreBytes = 1024
	seg, err := newSegment(dir, 0, config)
	if err != nil {
		t.Errorf("newSegment() error %v", err.Error())
	}

	tests := []struct {
		arg       []byte
		wantValue []byte
	}{
		{
			arg:       []byte("hello"),
			wantValue: []byte("hello"),
		},
		{
			arg:       []byte("world"),
			wantValue: []byte("world"),
		},
	}

	for i, tt := range tests {
		record := &api.Record{
			Value: tt.arg,
		}

		offset, err := seg.Append(record)
		if err != nil {
			t.Errorf("Append() error %v", err.Error())
		}
		if offset != uint64(i) {
			t.Errorf("want %v, got %v", i, offset)
		}

		got, err := seg.Read(offset)
		if err != nil {
			t.Errorf("Append() error %v", err.Error())
		}

		if bytes.Compare(tt.wantValue, got.Value) != 0 {
			t.Errorf("want %v, got %v", tt.wantValue, got)
		}
	}

	_, err = seg.Append(&api.Record{
		Value: []byte("fasfds"),
	})
	if err != io.EOF {
		t.Errorf("expected IO.EOF, got %v", err)
	}

	if !seg.isMaxed() {
		t.Errorf("isMaxed() exptected to be true")
	}

	seg, err = newSegment(dir, 0, config)
	if err != nil {
		t.Errorf("newSegment() error %v", err.Error())
	}
	if !seg.isMaxed() {
		t.Errorf("isMaxed() exptected to be true")
	}

	if err := seg.Remove(); err != nil {
		t.Errorf("Remove() error %v", err.Error())
	}

	seg, err = newSegment(dir, 0, config)
	if err != nil {
		t.Errorf("newSegment() error %v", err.Error())
	}

	if seg.isMaxed() {
		t.Errorf("isMaxed() exptected to be false")
	}
}
