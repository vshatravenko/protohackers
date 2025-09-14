package main

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/vshatravenko/protohackers/internal/server"
)

var payloads = []payload{
	{action: 'I', segment1: 12345, segment2: 101},
	{action: 'I', segment1: 12346, segment2: 102},
	{action: 'I', segment1: 12346, segment2: 100},
	{action: 'I', segment1: 40960, segment2: 5},
	{action: 'Q', segment1: 12288, segment2: 16384},
}

const expectedMean = int32(101)

// Start the TCP server and send a few test payloads using I action,
// then validate the resulting mean value
func TestMain(t *testing.T) {
	addr := &net.TCPAddr{IP: server.LocalIP, Port: server.DefaultPort}

	go main()
	t.Logf("Waiting for server at %s to start", addr.String())
	time.Sleep(1 * time.Second) // FIXME: wait for the port to be open instead

	client, err := net.DialTCP("tcp4", nil, addr)
	if err != nil {
		t.Errorf("Local client creation failed: %v", err)
	}

	for _, p := range payloads {
		t.Logf("Sending test payload %v", p)
		_, err := client.Write(p.toBytes())
		if err != nil {
			t.Errorf("Failed to send the payload: %v", err)
		}
	}

	respBuf := make([]byte, 4)
	_, err = client.Read(respBuf)
	if err != nil {
		t.Errorf("Failed to read the response: %v", err)
	}

	actualMean := int32(binary.BigEndian.Uint32(respBuf))
	if actualMean != expectedMean {
		t.Errorf("Actual mean %v is different from expected %v", actualMean, expectedMean)
	}
}
