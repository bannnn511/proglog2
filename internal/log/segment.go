package log

import (
	"fmt"
	"google.golang.org/protobuf/proto"
	"os"
	"path"

	api "proglog/api/v1"
)

// segment wraps index and store types
// baseOffset and nextOffset to calculate relative offset of active segment.
type segment struct {
	store      *store
	index      *index
	baseOffset uint64
	nextOffset uint64
	config     Config
}

// newSegment create index file and store file.
func newSegment(dir string, baseOffset uint64, config Config) (*segment, error) {
	s := &segment{
		baseOffset: baseOffset,
		config:     config,
	}

	storeFile, err := os.OpenFile(
		path.Join(dir, fmt.Sprintf("%d%s", baseOffset, ".store")),
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0644)
	if err != nil {
		return nil, err
	}

	s.store, err = newLogStore(storeFile)
	if err != nil {
		return nil, err
	}

	indexFile, err := os.OpenFile(
		path.Join(dir, fmt.Sprintf("%d%s", baseOffset, ".index")),
		os.O_RDWR|os.O_CREATE,
		0644)
	if err != nil {
		return nil, err
	}

	s.index, err = newIndex(indexFile, config)
	if err != nil {
		return nil, err
	}

	if offset, _, err := s.index.Read(-1); err != nil {
		s.nextOffset = baseOffset
	} else {
		s.nextOffset = baseOffset + uint64(offset) + 1
	}

	return s, nil
}

// Append appends record to the store of active segment and
// writes record offset and position to the index file.
func (s *segment) Append(record *api.Record) (uint64, error) {
	curOffset := s.nextOffset
	record.Offset = curOffset
	p, err := proto.Marshal(record)
	if err != nil {
		return 0, err
	}

	_, pos, err := s.store.Append(p)
	if err != nil {
		return 0, err
	}

	if err = s.index.Write(uint32(s.nextOffset-s.baseOffset), pos); err != nil {
		return 0, err
	}

	s.nextOffset++

	return curOffset, nil
}

// Read gets record position from index then
// fetch record from the store.
func (s *segment) Read(offset uint64) (*api.Record, error) {
	_, pos, err := s.index.Read(int64(offset - s.baseOffset))
	if err != nil {
		return nil, err
	}

	read, err := s.store.Read(pos)
	if err != nil {
		return nil, err
	}

	record := &api.Record{}
	err = proto.Unmarshal(read, record)
	if err != nil {
		return nil, err
	}

	return record, nil
}

// isMaxed checks if index or store file size exceeds config.
func (s *segment) isMaxed() bool {
	return s.index.size >= s.config.MaxIndexBytes ||
		s.store.size >= s.config.MaxStoreBytes
}

// Remove closes segment and
// remove index and store file.
func (s *segment) Remove() error {
	if err := s.Close(); err != nil {
		return err
	}

	if err := os.Remove(s.index.Name()); err != nil {
		return err
	}

	if err := os.Remove(s.store.Name()); err != nil {
		return err
	}

	return nil
}

// Close closes index and store file.
func (s *segment) Close() error {
	if err := s.index.Close(); err != nil {
		return err
	}

	if err := s.store.Close(); err != nil {
		return err
	}

	return nil
}
