package log

import (
	"os"
	"path"
	api "proglog/api/v1"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Record struct {
	Value  []byte `json:"value"`
	Offset uint64 `json:"offset"`
}

// Log TODO: implement multi-reader for log.
type Log struct {
	mu sync.RWMutex

	dir    string
	config Config

	segments      []*segment
	activeSegment *segment
}

// NewLog creates a Log instance and
// setup defaults if the caller didn't specify config.
func NewLog(dir string, config Config) (*Log, error) {
	if config.MaxIndexBytes == 0 {
		config.MaxIndexBytes = 1024
	}

	if config.MaxStoreBytes == 0 {
		config.MaxStoreBytes = 1024
	}

	log := &Log{
		dir:    dir,
		config: config,
	}

	return log, log.setup()
}

// setup responsible for setting itself up for segments that are already existed on disk.
// If the Log is new then setting up new segment.
func (l *Log) setup() error {
	files, err := os.ReadDir(l.dir)
	if err != nil {
		return err
	}

	var baseOffsets []uint64
	for _, file := range files {
		// Todo: check duplicate index
		offStr := strings.TrimSuffix(
			file.Name(),
			path.Ext(file.Name()),
		)
		off, _ := strconv.ParseUint(offStr, 10, 0)
		baseOffsets = append(baseOffsets, off)
	}

	sort.Slice(baseOffsets, func(i, j int) bool {
		return baseOffsets[i] < baseOffsets[j]
	})

	for _, baseOffset := range baseOffsets {
		if err := l.newSegment(uint64(baseOffset)); err != nil {
			return err
		}
	}

	if l.segments == nil {
		if err := l.newSegment(l.config.InitialOffset); err != nil {
			return err
		}
	}

	return nil
}

// newSegments creates new segment and appends to log's segments then
// makes the new segment active segment.
func (l *Log) newSegment(offset uint64) error {
	s, err := newSegment(l.dir, offset, l.config)
	if err != nil {
		return err
	}

	l.segments = append(l.segments, s)
	l.activeSegment = s

	return nil
}

// Append appends a record to the log and return that record offset
func (l *Log) Append(record *api.Record) (uint64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	off, err := l.activeSegment.Append(record)
	if err != nil {
		return 0, err
	}

	if l.activeSegment.isMaxed() {
		err := l.newSegment(off + 1)
		if err != nil {
			return 0, err
		}
	}

	return off, nil
}

// Read gets the record with the provided offset
// if offset is higher than the log records length
// return ErrorOffsetNotFound
func (l *Log) Read(offset uint64) (*api.Record, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var s *segment

	for _, segment := range l.segments {
		if segment.baseOffset <= offset && offset < segment.nextOffset {
			s = segment
			break
		}
	}

	if s == nil || s.nextOffset <= offset {
		return nil, api.ErrorOffsetOfOutRange{Offset: offset}
	}

	return s.Read(offset)
}

// LowestOffset returns the first segment base offset.
func (l *Log) LowestOffset() (uint64, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.segments[0].baseOffset, nil
}

// HighestOffset returns the last segment next offset - 1.
func (l *Log) HighestOffset() (uint64, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	off := l.segments[len(l.segments)-1].nextOffset
	if off == 0 {
		return 0, nil
	}

	return off - 1, nil
}

// Close closes all segments in the log.
func (l *Log) Close() error {
	for _, segment := range l.segments {
		if err := segment.Close(); err != nil {
			return err
		}
	}

	return nil
}

// Remove removes everything in log directory.
func (l *Log) Remove() error {
	if err := l.Close(); err != nil {
		return err
	}

	return os.RemoveAll(l.dir)
}

// Truncate removes all segments whose highest offset is lower than lowest.
func (l *Log) Truncate(lowest uint64) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var segments []*segment
	for _, s := range l.segments {
		if s.nextOffset <= lowest+1 {
			if err := s.Remove(); err != nil {
				return err
			}
			continue
		}
		segments = append(segments, s)
	}

	l.segments = segments

	return nil
}
