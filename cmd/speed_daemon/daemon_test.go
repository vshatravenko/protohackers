package main

import (
	"fmt"
	"io"
	"log/slog"
	"net"
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
	testCamera1Plates = []plateMsg{
		{plate: "UN1X", timestamp: 0},
	}
	testCamera2Init = iAmCameraMsg{
		road:  123,
		mile:  9,
		limit: 60,
	}
	testCamera2Plates = []plateMsg{
		{plate: "UN1X", timestamp: 45},
	}
	testDispatcher1Init = iAmDispatcherMsg{
		numRoads: 2,
		roads:    []uint16{123, 256},
	}
	testDispatcher1Tickets = []*ticketMsg{
		{plate: "UN1X", road: 123, mile1: 8, timestamp1: 0, mile2: 9, timestamp2: 45, speed: 8000},
	}
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
	go runTestCamera(t, testCamera1Init, testCamera1Plates)
	go runTestCamera(t, testCamera2Init, testCamera2Plates)
	go runTestDispatcher(t, testDispatcher1Init, testDispatcher1Tickets)
	time.Sleep(60 * time.Second)
}

func runTestCamera(t *testing.T, initMsg iAmCameraMsg, plates []plateMsg) {
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

	for _, plate := range plates {
		slog.Info("test camera sending plate msg", "id", id, "msg", plate, "payload", plate.Bytes())
		_, err = conn.Write(plate.Bytes())
		if err != nil {
			t.Errorf("test camera failed to send the plate message, id: %s, err: %s, plate: %v", id, err.Error(), plate)
			return
		}

	}

	// Keep the connection open
	b := make([]byte, bufSize)
	_, _ = conn.Read(b)
}

func runTestDispatcher(t *testing.T, initMsg iAmDispatcherMsg, expectedTickets []*ticketMsg) {
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
			actual := parseTicketMsg(payload[1:])
			slog.Info("test dispatcher parsed ticket msg", "id", id, "ticket", actual)
			expected := expectedTickets[0]
			expectedTickets = expectedTickets[1:]
			if isEqualTicket(expected, actual) {
				slog.Info("ticket equality check passed", "id", id, "ticket", actual)
			} else {
				t.Errorf("ticket equality check failed:\nactual:%v\nexpected: %v", actual, expected)
			}

		case msgTypes["heartbeat"]:
			slog.Info("test dispatcher received heartbeat", "id", id)
		}

	}

}

func isEqualTicket(expected, actual *ticketMsg) bool {
	plate := expected.plate == actual.plate
	road := expected.road == actual.road
	mile1 := expected.mile1 == actual.mile1
	ts1 := expected.timestamp1 == actual.timestamp1
	mile2 := expected.mile2 == actual.mile2
	ts2 := expected.timestamp2 == actual.timestamp2
	speed := expected.speed == actual.speed

	slog.Info("isEqualTicket results", "plate", plate, "road", road, "mile1", mile1, "ts1", ts1, "mile2", mile2, "ts2", ts2, "speed", speed)

	return plate && road && mile1 && ts1 && mile2 && ts2 && speed
}
