package backend

import (
	"fmt"
	"reflect"
	"testing"

	"go.acuvity.ai/minibridge/pkgs/auth"
)

func Test_makeMCPError(t *testing.T) {
	type args struct {
		ID  any
		err error
	}
	tests := []struct {
		name string
		args func(t *testing.T) args

		want1 []byte
	}{
		{
			"basic",
			func(*testing.T) args {
				return args{
					ID:  42,
					err: fmt.Errorf("oh noes!"),
				}
			},
			[]byte(`{"error":{"code":451,"message":"oh noes!"},"id":42,"jsonrpc":"2.0"}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1 := makeMCPError(tArgs.ID, tArgs.err)

			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("makeMCPError got1 = %v, want1: %v", string(got1), string(tt.want1))
			}
		})
	}
}

func Test_parseBasicAuth(t *testing.T) {
	type args struct {
		authString string
	}
	tests := []struct {
		name string
		args func(t *testing.T) args

		want1 *auth.Auth
		want2 bool
	}{
		{
			"empty header",
			func(t *testing.T) args {
				return args{
					authString: "",
				}
			},
			nil,
			false,
		},
		{
			"bearer",
			func(t *testing.T) args {
				return args{
					authString: "Bearer token",
				}
			},
			auth.NewBearerAuth("token"),
			true,
		},
		{
			"basic",
			func(t *testing.T) args {
				return args{
					authString: "Basic dXNlcjpwYXNz",
				}
			},
			auth.NewBasicAuth("user", "pass"),
			true,
		},
		{
			"invalid basic b64",
			func(t *testing.T) args {
				return args{
					authString: "Basic not-b64",
				}
			},
			nil,
			false,
		},
		{
			"invalid basic decoded",
			func(t *testing.T) args {
				return args{
					authString: "Basic aGVsbG8=",
				}
			},
			nil,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1, got2 := parseBasicAuth(tArgs.authString)

			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("parseBasicAuth got1 = %v, want1: %v", got1, tt.want1)
			}

			if !reflect.DeepEqual(got2, tt.want2) {
				t.Errorf("parseBasicAuth got2 = %v, want2: %v", got2, tt.want2)
			}
		})
	}
}
