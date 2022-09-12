package log

import (
	"bufio"
	"encoding/binary"
	"os"
	"sync"
)

var (
	enc = binary.BigEndian
)

const (
	lenWidth = 8
)

type store struct {
	*os.File
	mu   sync.Mutex
	buf  *bufio.Writer
	size uint64
}

// newStore creates a file to store records.
func newStore(f *os.File) (*store, error) {
	file, err := os.Stat(f.Name())
	if err != nil {
		return nil, err
	}

	return &store{
		File: f,
		buf:  bufio.NewWriter(f),
		size: uint64(file.Size()),
	}, nil
}

// Append writes data to file and returns record length, position and error.
// The first 8 bytes store the length of record so that we know how many bytes to read
// then we write data to buffer and increment store size = size of data + length of record.
func (s *store) Append(data []byte) (uint64, uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pos := s.size
	if err := binary.Write(s.buf, enc, uint64(len(data))); err != nil {
		return 0, 0, err
	}

	nn, err := s.buf.Write(data)
	if err != nil {
		return 0, 0, err
	}

	nn += lenWidth
	s.size += uint64(nn)
	return uint64(nn), pos, nil
}

func (s *store) Read(pos uint64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.buf.Flush(); err != nil {
		return nil, err
	}

	size := make([]byte, lenWidth)
	_, err := s.File.ReadAt(size, int64(pos))
	if err != nil {
		return nil, err
	}

	data := make([]byte, enc.Uint64(size))
	_, err = s.File.ReadAt(data, int64(pos+lenWidth))
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (s *store) ReadAt(p []byte, pos int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.buf.Flush(); err != nil {
		return 0, err
	}

	return s.File.ReadAt(p, pos)
}

func (s *store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.buf.Flush(); err != nil {
		return err
	}

	return s.File.Close()
}
