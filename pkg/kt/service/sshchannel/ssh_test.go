package sshchannel

import (
	"errors"
	"net"
	"testing"
	"time"
)

func TestHandleRequestDoesNotSerializeSlowLocalDials(t *testing.T) {
	listener := &queuedListener{connections: make(chan net.Conn, 3)}
	clients := make([]net.Conn, 0, 3)
	for i := 0; i < 3; i++ {
		client, server := net.Pipe()
		clients = append(clients, client)
		listener.connections <- server
	}
	defer func() {
		for _, client := range clients {
			_ = client.Close()
		}
	}()

	dialStarted := make(chan struct{}, 3)
	dialTimeouts := make(chan time.Duration, 3)
	releaseDial := make(chan struct{})
	dial := func(_ string, _ string, timeout time.Duration) (net.Conn, error) {
		dialStarted <- struct{}{}
		dialTimeouts <- timeout
		<-releaseDial
		return nil, errors.New("local service unavailable")
	}

	loopDone := make(chan error, 1)
	go func() {
		for i := 0; i < 3; i++ {
			if err := handleRequestWithDial(listener, "127.0.0.1:81", dial); err != nil {
				loopDone <- err
				return
			}
		}
		loopDone <- nil
	}()

	for i := 0; i < 3; i++ {
		select {
		case <-dialStarted:
		case <-time.After(time.Second):
			t.Fatalf("only %d local dials started while earlier dials were blocked", i)
		}
		if timeout := <-dialTimeouts; timeout != localDialTimeout {
			t.Fatalf("local dial timeout = %s, want %s", timeout, localDialTimeout)
		}
	}
	select {
	case err := <-loopDone:
		if err != nil {
			t.Fatalf("handleRequestWithDial() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("accept loop waited for blocked local dials")
	}
	close(releaseDial)
}

type queuedListener struct {
	connections chan net.Conn
}

func (l *queuedListener) Accept() (net.Conn, error) {
	return <-l.connections, nil
}

func (l *queuedListener) Close() error { return nil }

func (l *queuedListener) Addr() net.Addr { return pipeAddr("queued-listener") }

type pipeAddr string

func (a pipeAddr) Network() string { return string(a) }

func (a pipeAddr) String() string { return string(a) }

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
