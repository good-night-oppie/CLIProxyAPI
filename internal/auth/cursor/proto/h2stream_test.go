package proto

import (
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/http2"
)

func TestCloseUnblocksReaderAndFlowControlledWriter(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	stream := &H2Stream{
		conn: client, framer: http2.NewFramer(client, client), streamID: 1,
		dataCh: make(chan []byte), doneCh: make(chan struct{}), closeCh: make(chan struct{}),
	}
	stream.windowCond = sync.NewCond(&stream.windowMu)
	go stream.readLoop()
	writerDone := make(chan error, 1)
	go func() { writerDone <- stream.Write([]byte("blocked by flow control")) }()

	// An unbuffered data channel blocks the reader after this frame is read.
	framer := http2.NewFramer(server, server)
	if err := framer.WriteData(1, false, []byte("unconsumed")); err != nil {
		t.Fatal(err)
	}
	stream.Close()
	stream.Close() // Closing a stream must be idempotent.
	select {
	case <-stream.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("reader did not exit after Close")
	}
	select {
	case err := <-writerDone:
		if !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("Write error = %v, want closed pipe", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("flow-controlled writer did not exit after Close")
	}
}
