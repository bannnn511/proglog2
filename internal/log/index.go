package log

import (
	"github.com/tysonmote/gommap"
	"io"
	"os"
)

const (
	offWidth uint64 = 4 // Record offset
	posWidth uint64 = 8 // Record position
)

var (
	entryWidth = offWidth + posWidth
)

// index defines our index file which compromises of
// a persisted file and a memory-mapped mmap
// size tells us the size of index and
// where to write the next entry to appended to the index.
type index struct {
	file *os.File
	mmap gommap.MMap
	size uint64
}

// newIndex creates an index for a given file.
func newIndex(f *os.File, c Config) (*index, error) {
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	idx := &index{
		file: f,
		size: uint64(fi.Size()),
	}

	// truncate to max size before memory-mapped
	err = f.Truncate(int64(c.Segment.MaxIndexBytes))
	if err != nil {
		return nil, err
	}

	// memory-mapping file
	if idx.mmap, err = gommap.Map(
		f.Fd(),
		gommap.PROT_READ|gommap.PROT_WRITE,
		gommap.MAP_SHARED,
	); err != nil {
		return nil, err
	}

	return idx, nil
}

// Close syncs memory-mapped file to persistent file
// then truncates the persistent file to size of data that actually in it
// and closes the file.
func (i *index) Close() error {
	err := i.mmap.Sync(gommap.MS_SYNC)
	if err != nil {
		return err
	}

	err = i.file.Sync()
	if err != nil {
		return err
	}

	err = i.file.Truncate(int64(i.size))
	if err != nil {
		return err
	}

	return i.file.Close()
}

// Write appends the given offset and position to the index.
func (i *index) Write(off uint32, position uint64) error {
	// validate if we have size to write the entry
	if uint64(len(i.mmap)) < i.size+entryWidth {
		return io.EOF
	}

	enc.PutUint32(i.mmap[i.size:i.size+offWidth], off)                 // encode offset
	enc.PutUint64(i.mmap[i.size+offWidth:i.size+entryWidth], position) // encode position
	i.size += entryWidth
	return nil
}

// Read takes an offset
// and return the associated record's position in the store.
func (i *index) Read(in int64) (out uint32, pos uint64, err error) {
	if i.size == 0 {
		return 0, 0, io.EOF
	}

	if in == -1 {
		out = uint32((i.size / entryWidth) - 1)
	} else {
		out = uint32(in)
	}

	pos = uint64(out) * entryWidth

	if i.size <= pos {
		return 0, 0, io.EOF
	}

	out = enc.Uint32(i.mmap[pos : pos+offWidth])            // read record's offset
	pos = enc.Uint64(i.mmap[pos+offWidth : pos+entryWidth]) // read record's position

	return out, pos, nil
}

// Name returns index persistent file's name.
func (i *index) Name() string { return i.file.Name() }
