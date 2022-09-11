package log

import (
	"reflect"
	"sync"
	"testing"
)

func TestLog_Append(t *testing.T) {
	type fields struct {
		records []Record
	}

	type args struct {
		record Record
	}

	tests := []struct {
		name   string
		fields fields
		args   []args
		want   uint64
	}{
		{
			name: "Test append one record",
			fields: fields{
				records: []Record{},
			},
			args: []args{{
				record: Record{},
			}},
			want: 0,
		},
		{
			name: "Test append two records",
			fields: fields{
				records: []Record{},
			},
			args: []args{
				{
					record: Record{},
				},
				{
					record: Record{},
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Log{
				records: tt.fields.records,
				mu:      sync.Mutex{},
			}

			var got uint64 = 0
			for _, arg := range tt.args {
				got, _ = l.Append(arg.record)
			}

			if got != tt.want {
				t.Errorf("Append() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLog_Read(t *testing.T) {
	type fields struct {
		records []Record
	}
	type args struct {
		offset uint64
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    Record
		wantErr error
	}{
		{
			name: "Test Read at offset 0",
			fields: fields{
				records: generateDummyRecords(),
			},
			args:    args{offset: 0},
			want:    Record{Offset: 0, Value: []byte("test 0")},
			wantErr: ErrorOffsetNotFound,
		},
		{
			name: "Test Read at offset 1",
			fields: fields{
				records: generateDummyRecords(),
			},
			args:    args{offset: 1},
			want:    Record{Offset: 1, Value: []byte("test 1")},
			wantErr: ErrorOffsetNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Log{
				records: tt.fields.records,
				mu:      sync.Mutex{},
			}
			got, err := l.Read(tt.args.offset)
			if err != nil && err != tt.wantErr {
				t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Read() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func generateDummyRecords() []Record {
	return []Record{
		{
			Value:  []byte("test 0"),
			Offset: 0,
		},
		{
			Value:  []byte("test 1"),
			Offset: 1,
		},
	}
}
