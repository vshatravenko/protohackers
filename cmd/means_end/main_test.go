package main

import (
	"encoding/binary"
	"math/rand/v2"
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

const (
	expectedInitMean   = int32(101)
	randomPayloadCount = 200000
)

// Start the TCP server and send a few test payloads using I action,
// then validate the resulting mean value
func TestMain(t *testing.T) {
	addr := &net.TCPAddr{IP: server.LocalIP, Port: server.DefaultPort}

	go main()
	t.Logf("Waiting for server at %s to start", addr.String())
	time.Sleep(1 * time.Second) // FIXME: wait for the port to be open instead

	conn, err := net.DialTCP("tcp4", nil, addr)
	if err != nil {
		t.Errorf("Local client creation failed: %v", err)
	}

	for _, p := range payloads {
		t.Logf("Sending test payload %v", p)
		_, err := conn.Write(p.toBytes())
		if err != nil {
			t.Errorf("Failed to send the payload: %v", err)
		}
	}

	respBuf := make([]byte, 4)
	_, err = conn.Read(respBuf)
	if err != nil {
		t.Errorf("Failed to read the response: %v", err)
	}

	actualMean := int32(binary.BigEndian.Uint32(respBuf))
	if actualMean != expectedInitMean {
		t.Errorf("Actual mean %v is different from expected %v", actualMean, expectedInitMean)
	}

	t.Logf("Inserting %d random payloads", randomPayloadCount)
	insertRandomPayloads(t, conn, randomPayloadCount)

	for _, p := range payloads {
		t.Logf("Sending test payload %v", p)
		_, err := conn.Write(p.toBytes())
		if err != nil {
			t.Errorf("Failed to send the payload: %v", err)
		}
	}
}

func insertRandomPayloads(t *testing.T, conn *net.TCPConn, count int) {
	buf := []byte{}

	for range count {
		price, date := rand.Int32(), rand.Int32()
		if price < 0 {
			price *= -1
		}

		if date < 0 {
			date *= -1
		}

		p := payload{
			action:   'I',
			segment1: date,
			segment2: price,
		}

		buf = append(buf, p.toBytes()...)
	}

	_, err := conn.Write(buf)
	if err != nil {
		t.Errorf("Failed to send payload: %v", err)
	}
}
