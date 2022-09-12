package log

import (
	"fmt"
	"os"
	"testing"
)

func Test_store_Append(t *testing.T) {
	file, _ := os.CreateTemp("", "store_append_test")
	s, _ := newStore(file)

	type args struct {
		data []byte
	}
	tests := []struct {
		name             string
		args             args
		wantRecordLength uint64
		wantPos          uint64
	}{
		{
			name: "test write hello",
			args: args{
				data: []byte("hello"),
			},
			wantRecordLength: 13,
			wantPos:          0,
		},
		{
			name: "test write word",
			args: args{
				data: []byte(" world"),
			},
			wantRecordLength: 14,
			wantPos:          13,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recordLength, pos, err := s.Append(tt.args.data)
			if err != nil {
				t.Errorf("Append() error = %v", err)
				return
			}
			if recordLength != tt.wantRecordLength {
				t.Errorf("Append() recordLength = %v, wantRecordLength %v", recordLength, tt.wantRecordLength)
			}
			if pos != tt.wantPos {
				t.Errorf("Append() pos = %v, wantPos %v", pos, tt.wantPos)
			}
		})
	}
}

func Test_store_Append_Read(t *testing.T) {
	content := "test"
	file, _ := os.CreateTemp("", "store_append_test")
	defer file.Close()

	s, _ := newStore(file)

	u, u2, err := s.Append([]byte(content))
	if err != nil {
		panic(err)
	}
	fmt.Println(u, u2)

	data, err := s.Read(u2)
	if err != nil {
		panic(err)
	}

	if string(data) != content {
		t.Errorf("want %v, got %v", content, string(data))
	}

	fmt.Printf("got: %q, want: %v\n", data, content)
}

func Test_store_ReadAt(t *testing.T) {
	content := "abcd"
	file, _ := os.CreateTemp("", "store_append_test")
	defer file.Close()

	s, _ := newStore(file)

	u, u2, err := s.Append([]byte(content))
	if err != nil {
		panic(err)
	}
	fmt.Println("u", u, "u2", u2)

	_, u1, _ := s.Append([]byte(content))
	fmt.Println("u1", u1)
	b := make([]byte, u)
	test, err := s.ReadAt(b, int64(u1-1))
	if err != nil {
		panic(err)
	}

	fmt.Printf("readAt %q\n", test)

	data, err := s.Read(u2)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(data))

	if string(data) != content {
		t.Errorf("want %v, got %v", content, string(data))
	}
}
