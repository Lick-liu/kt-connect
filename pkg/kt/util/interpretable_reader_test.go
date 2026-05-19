package util

import (
	"io"
	"testing"
	"time"
)

func TestInterpretableReaderCancelDoesNotBlockAfterReadCompletes(t *testing.T) {
	reader, writer := io.Pipe()
	interruptible := NewInterpretableReader(reader)

	readDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, interruptible)
		close(readDone)
	}()

	_ = writer.Close()
	select {
	case <-readDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("reader did not finish after source closed")
	}

	cancelDone := make(chan struct{})
	go func() {
		interruptible.Cancel()
		close(cancelDone)
	}()

	select {
	case <-cancelDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Cancel blocked after reader had already completed")
	}
}
