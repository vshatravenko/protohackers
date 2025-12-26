package main

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/vshatravenko/protohackers/internal/server"
)

var (
	serverAddr      = &net.TCPAddr{IP: server.LocalIP, Port: server.DefaultPort}
	testCamera1Init = iAmCameraMsg{
		road:  123,
		mile:  8,
		limit: 60,
	}
	testCamera1Hb = wantHeartbeatMsg{
		interval: 10, // 1 second
	}
	testCamera1Plates = []plateMsg{
		{plate: "UN1X", timestamp: 0},
		{plate: "UN1X", timestamp: 100},
	}
	testCamera2Init = iAmCameraMsg{
		road:  123,
		mile:  9,
		limit: 60,
	}
	testCamera2Hb = wantHeartbeatMsg{
		interval: 25, // 2.5 seconds
	}
	testCamera2Plates = []plateMsg{
		{plate: "UN1X", timestamp: 45},
		{plate: "UN1X", timestamp: 145},
	}
	testDispatcher1Init = iAmDispatcherMsg{
		numRoads: 2,
		roads:    []uint16{123, 224},
	}
	testDispatcher1Hb = wantHeartbeatMsg{
		interval: 30, // 3 seconds
	}
	testDispatcher1Tickets = []*ticket{
		{plate: "UN1X", road: 123, mile1: 8, timestamp1: 0, mile2: 9, timestamp2: 45, speed: 8000},
	}
	testTimeout = 15 * time.Second
)

/*
1. Start the main server
2. Create TCP clients for cameras and dispatchers
3. Send a few payloads
4. Validate the response
*/
func TestDaemon(t *testing.T) {
	// TODO: add a wait group for each component to stop as soon as the test is completed
	go main()
	time.Sleep(1 * time.Second)
	go runTestCamera(t, testCamera1Init, testCamera1Hb, testCamera1Plates)
	go runTestCamera(t, testCamera2Init, testCamera2Hb, testCamera2Plates)
	wg := new(sync.WaitGroup)
	wg.Add(len(testDispatcher1Tickets))
	go runTestDispatcher(t, testDispatcher1Init, testDispatcher1Hb, testDispatcher1Tickets, wg)

	done := make(chan int)
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("Test completed successfully on time")
		return
	case <-time.After(testTimeout):
		t.Errorf("The test didn't complete after 15 seconds")
		break
	}
}

func runTestCamera(t *testing.T, initMsg iAmCameraMsg, hbMsg wantHeartbeatMsg, plates []plateMsg) {
	id := fmt.Sprintf("#%d-%d", initMsg.road, initMsg.mile)
	slog.Info("test camera dialing", "id", id)
	conn, err := net.DialTCP("tcp4", nil, serverAddr)
	if err != nil {
		t.Errorf("test camera failed to open connection, id: %s, err: %s", id, err.Error())
	}

	slog.Info("test camera sending init msg", "id", id)
	_, err = conn.Write(initMsg.Bytes())
	if err != nil {
		t.Errorf("test camera failed to send the init message, id: %s, err: %s", id, err.Error())
	}

	slog.Info("test camera sending want_heartbeat msg", "id", id)
	_, err = conn.Write(hbMsg.Bytes())
	if err != nil {
		t.Errorf("test camera failed to send want_heartbeat msg, id: %s, err: %s", id, err.Error())
	}

	for _, plate := range plates {
		slog.Info("test camera sending plate msg", "id", id, "msg", plate, "payload", plate.Bytes())
		_, err = conn.Write(plate.Bytes())
		if err != nil {
			t.Errorf("test camera failed to send the plate message, id: %s, err: %s, plate: %v", id, err.Error(), plate)
		}
	}

	// Keep the connection open
	b := make([]byte, bufSize)
	_, _ = conn.Read(b)
}

func runTestDispatcher(t *testing.T, initMsg iAmDispatcherMsg, hbMsg wantHeartbeatMsg, expectedTickets []*ticket, wg *sync.WaitGroup) {
	id := fmt.Sprintf("#%d-%d", initMsg.roads[0], initMsg.numRoads)
	slog.Info("test dispatcher dialing", "id", id)
	conn, err := net.DialTCP("tcp4", nil, serverAddr)
	if err != nil {
		t.Errorf("test dispatcher failed to open conn, id: %s, err: %s", id, err.Error())
	}
	defer closeConn(conn)

	slog.Info("test dispatcher sending init msg", "id", id)
	_, err = conn.Write(initMsg.Bytes())
	if err != nil {
		t.Errorf("test dispatcher failed to send init msg, id: %s, err: %s", id, err.Error())
	}

	slog.Info("test dispatcher sending want_heartbeat msg", "id", id)
	_, err = conn.Write(hbMsg.Bytes())
	if err != nil {
		t.Errorf("test dispatcher failed to send want_heartbeat msg, id: %s, err: %s", id, err.Error())
	}

	slog.Info("test dispatcher listening for messages", "id", id)
	for {
		b := make([]byte, bufSize)
		n, err := conn.Read(b)
		if err != nil && err != io.EOF {
			t.Errorf("test dispatcher failed to read msg, id: %s, err: %s", id, err.Error())
		}

		payload := b[:n]

		slog.Info("test dispatcher received msg", "id", id)
		msgType := payload[0]
		switch msgType {
		case msgTypes["ticket"]:
			slog.Info("test dispatcher received ticket msg", "id", id)
			actual := parseTicket(payload)
			slog.Info("test dispatcher parsed ticket msg", "id", id, "ticket", actual)
			expected := expectedTickets[0]
			expectedTickets = expectedTickets[1:]
			if expected.equal(actual) {
				slog.Info("ticket equality check passed", "id", id, "ticket", actual)
			} else {
				t.Errorf("ticket equality check failed:\nactual:%v\nexpected: %v", actual, expected)
			}
			wg.Done()

		case msgTypes["heartbeat"]:
			slog.Info("test dispatcher received heartbeat", "id", id)
		default:
			t.Errorf("test dispatcher %s received unexpected msg type - 0x%X", id, msgType)
		}

	}

}

func TestSpeed(t *testing.T) {
	const (
		mile1    = 8
		mile2    = 9
		ts1      = 0
		ts2      = 45
		expected = 8000
	)

	actual := calculateSpeed(mile1, mile2, ts1, ts2)
	if actual != expected {
		t.Errorf("Speed:\nexpected: %d\nactual: %d", expected, actual)
	}
}
