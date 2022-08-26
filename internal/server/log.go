package server

import (
	"errors"
	"sync"
)

var ErrorOffsetNotFound = errors.New("offset not found")

type Record struct {
	Value  []byte
	Offset uint64
}

type Log struct {
	records []Record
	mu      sync.Mutex
}

// Append appends a record to the log and return that record offset
func (l *Log) Append(record Record) (uint64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	record.Offset = uint64(len(l.records))
	l.records = append(l.records, record)

	return record.Offset, nil
}

// Read gets the record with the provided offset
// if offset is higher than the log records length
// return ErrorOffsetNotFound
func (l *Log) Read(offset uint64) (Record, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if offset > uint64(len(l.records)) {
		return Record{}, ErrorOffsetNotFound
	}

	return l.records[offset], nil
}
