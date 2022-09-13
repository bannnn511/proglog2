package log

import (
	"io"
	"os"
	"testing"
)

// var tempFile *os.File
var config Config

func TestMain(m *testing.M) {
	//var err error
	//
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}

	code := m.Run()

	os.Exit(code)
}

func Test_index_Write_Read(t *testing.T) {
	tempFile, err := os.CreateTemp(os.TempDir(), "index_text")
	config.MaxIndexBytes = 1024
	defer os.Remove(tempFile.Name())

	type args struct {
		off      uint32
		position uint64
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test 1st write",
			args: args{
				off:      0,
				position: 0,
			},
		},
		{
			name: "test 2nd write",
			args: args{
				off:      1,
				position: 10,
			},
		},
	}

	// test build index
	i, err := newIndex(tempFile, config)
	if err != nil {
		t.Errorf("newIndex() error = %v", err.Error())
	}

	// test file name
	if i.Name() != tempFile.Name() {
		t.Errorf("Name() error, got %v want %v", i.Name(), tempFile.Name())
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// test write
			if err := i.Write(tt.args.off, tt.args.position); err != nil {
				t.Errorf("Write() error = %v", err)
			}

			// test read
			_, pos, _ := i.Read(int64(tt.args.off))
			if pos != tt.args.position {
				t.Errorf("Read() error, got %v want %v", pos, tt.args.position)
			}
		})
	}

	// index should be error if reading past existing entries
	_, _, err = i.Read(int64(len(tests)))
	if err != io.EOF {
		t.Errorf("expect EOF %v", err)
	}

	// test close file
	err = i.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err.Error())
	}

	f, _ := os.OpenFile(tempFile.Name(), os.O_RDWR, 0600)
	// index should build its state from existing file.
	i2, err := newIndex(f, config)
	if err != nil {
		t.Errorf("newIndex() error = %v", err.Error())
	}

	out, pos, err := i2.Read(-1)
	if err != nil {
		t.Errorf("Read() error %v", err.Error())
	}

	if out != tests[1].args.off {
		t.Errorf("got %v, want %v", out, tests[1].args.off)
	}

	if pos != tests[1].args.position {
		t.Errorf("got %v, want %v", out, tests[1].args.position)
	}
}

func Test_Simple_Read_Write(t *testing.T) {
	tempFile, _ := os.CreateTemp(os.TempDir(), "index_text")
	config.MaxIndexBytes = 1024
	defer os.Remove(tempFile.Name())

	// test build index
	i, err := newIndex(tempFile, config)
	defer i.Close()
	if err != nil {
		t.Errorf("newIndex() error = %v", err.Error())
	}

	var wantOff uint32 = 0
	var wantPos uint64 = 10
	err = i.Write(wantOff, wantPos)
	if err != nil {
		t.Errorf("Write() error %v", err.Error())
	}

	out, pos, err := i.Read(int64(wantOff))
	if out != wantOff {
		t.Errorf("got %v want %v", out, wantOff)
	}

	if pos != wantPos {
		t.Errorf("got %v want %v", pos, wantPos)
	}

	_, _, err = i.Read(1)
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err.Error())
	}

	_, _, err = i.Read(2)
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err.Error())
	}
	io.MultiReader()
}
