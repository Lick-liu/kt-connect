package sshchannel

import (
	"errors"
	"net"
	"testing"
)

func TestShouldIgnoreCopyCloseError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "net err closed",
			err:  net.ErrClosed,
			want: true,
		},
		{
			name: "wrapped net err closed",
			err:  errors.New("read tcp 127.0.0.1:52866->127.0.0.1:81: use of closed network connection"),
			want: true,
		},
		{
			name: "real copy failure",
			err:  errors.New("connection reset by peer"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldIgnoreCopyError(tt.err); got != tt.want {
				t.Fatalf("shouldIgnoreCopyError() = %v, want %v", got, tt.want)
			}
		})
	}
}
