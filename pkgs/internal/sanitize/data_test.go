package sanitize

import (
	"reflect"
	"testing"
)

func TestSanitize(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name string
		args func(t *testing.T) args

		want1 []byte
	}{

		{
			"empty",
			func(*testing.T) args {
				return args{
					data: []byte(""),
				}
			},
			[]byte(""),
		},
		{
			"nil",
			func(*testing.T) args {
				return args{
					data: nil,
				}
			},
			nil,
		},
		{
			"no suffix",
			func(*testing.T) args {
				return args{
					data: []byte("hello world"),
				}
			},
			[]byte("hello world"),
		},
		{
			"with \n",
			func(*testing.T) args {
				return args{
					data: []byte("hello world\n\n\n\n\n"),
				}
			},
			[]byte("hello world"),
		},
		{
			"with \r",
			func(*testing.T) args {
				return args{
					data: []byte("hello world\r\r\r\r"),
				}
			},
			[]byte("hello world"),
		},
		{
			"with \n\r",
			func(*testing.T) args {
				return args{
					data: []byte("hello world\r\n"),
				}
			},
			[]byte("hello world"),
		},
		{
			"with \n\r at start",
			func(*testing.T) args {
				return args{
					data: []byte("\r\nhello world\r\n"),
				}
			},
			[]byte("\r\nhello world"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1 := Data(tArgs.data)

			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("Sanitize got1 = %v, want1: %v", got1, tt.want1)
			}
		})
	}
}
