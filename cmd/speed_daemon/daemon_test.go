package main

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/vshatravenko/protohackers/internal/logger"
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
		{plate: "LOL1X", timestamp: 125},
		{plate: "T3ST", timestamp: 150},
		{plate: "UN1X", timestamp: 300},
	}
	testCamera2Init = iAmCameraMsg{
		road:  123,
		mile:  9,
		limit: 60,
	}
	testCamera2Hb = wantHeartbeatMsg{
		interval: 0, // never
	}
	testCamera2Plates = []plateMsg{
		{plate: "UN1X", timestamp: 45},
		{plate: "LOL1X", timestamp: 126},
		{plate: "T3ST", timestamp: 160},
		{plate: "UN1X", timestamp: 1000},
	}
	testCamera3Init = iAmCameraMsg{
		road:  224,
		mile:  10,
		limit: 25,
	}
	testCamera3Hb = wantHeartbeatMsg{
		interval: 10, // 1 second
	}
	testCamera3Plates = []plateMsg{
		{plate: "MULT1", timestamp: 1767027383},
	}
	testCamera4Init = iAmCameraMsg{
		road:  224,
		mile:  1400,
		limit: 25,
	}
	testCamera4Hb = wantHeartbeatMsg{
		interval: 0, // never
	}
	testCamera4Plates = []plateMsg{
		{plate: "MULT1", timestamp: 1767200183},
	}
	testDispatcher1Init = iAmDispatcherMsg{
		numRoads: 2,
		roads:    []uint16{123, 255},
	}
	testDispatcher1Hb = wantHeartbeatMsg{
		interval: 30, // 3 seconds
	}
	testDispatcher1Tickets = []*ticket{
		{plate: "UN1X", road: 123, mile1: 8, timestamp1: 0, mile2: 9, timestamp2: 45, speed: 8000},
		{plate: "LOL1X", road: 123, mile1: 8, timestamp1: 125, mile2: 9, timestamp2: 126, speed: 32320},
		{plate: "T3ST", road: 123, mile1: 8, timestamp1: 150, mile2: 9, timestamp2: 160, speed: 36000},
	}
	testDispatcher2Init = iAmDispatcherMsg{
		numRoads: 1,
		roads:    []uint16{224},
	}
	testDispatcher2Hb = wantHeartbeatMsg{
		interval: 30, // 3 seconds
	}
	testDispatcher2Tickets = []*ticket{ // multiple tickets since it spans three days
		{plate: "MULT1", road: 224, mile1: 10, timestamp1: 1767027383, mile2: 1400, timestamp2: 1767052800, speed: 2895},
		{plate: "MULT1", road: 224, mile1: 10, timestamp1: 1767052800, mile2: 1400, timestamp2: 1767139200, speed: 2895},
		{plate: "MULT1", road: 224, mile1: 10, timestamp1: 1767139200, mile2: 1400, timestamp2: 1767200183, speed: 2895},
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
	go runTestCamera(t, testCamera3Init, testCamera3Hb, testCamera3Plates)
	go runTestCamera(t, testCamera4Init, testCamera4Hb, testCamera4Plates)
	wg := new(sync.WaitGroup)
	wg.Add(len(testDispatcher1Tickets))
	wg.Add(len(testDispatcher2Tickets))
	time.Sleep(time.Second) // simulate an unavailable dispatcher
	go runTestDispatcher(t, testDispatcher1Init, testDispatcher1Hb, testDispatcher1Tickets, wg)
	go runTestDispatcher(t, testDispatcher2Init, testDispatcher2Hb, testDispatcher2Tickets, wg)

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

	slog.Info("test camera sending want_heartbeat msg", "id", id)
	_, err = conn.Write(hbMsg.Bytes())
	if err != nil {
		t.Errorf("test camera failed to send want_heartbeat msg, id: %s, err: %s", id, err.Error())
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

		for len(payload) > 0 {
			slog.Info("test dispatcher processing payload", "id", id)
			msgType := payload[0]
			switch msgType {
			case msgTypes["ticket"]:
				slog.Info("test dispatcher received ticket msg", "id", id)
				actual := parseTicket(payload)
				slog.Info("test dispatcher parsed ticket msg", "id", id, "ticket", actual)
				found := false
				for _, t := range expectedTickets {
					if t.equal(actual) {
						slog.Info("test dispatcher received an expected ticket", "id", id, "ticket", actual)
						found = true
						wg.Done()
						break
					}
				}

				if !found {
					t.Errorf("test dispatcher %s received an unexpected ticket: %+v", id, actual)
				}

				offset := 1 + len(actual.plate) + 1 + 16
				payload = payload[offset:]
				slog.Debug("test dispatcher shifted the payload", "id", id, "payload", payload)

			case msgTypes["heartbeat"]:
				slog.Info("test dispatcher received heartbeat", "id", id)
				payload = payload[1:]
			default:
				t.Errorf("test dispatcher %s received unexpected msg type - 0x%X", id, msgType)
				return
			}
		}

	}

}

var speedTestCases = []*ticket{
	{timestamp1: 0, timestamp2: 45, mile1: 8, mile2: 9, speed: 8000},
	{timestamp1: 11565263, timestamp2: 11601479, mile1: 1016, mile2: 10, speed: 10000},
	{timestamp1: 1767027383, timestamp2: 1767200183, mile1: 10, mile2: 1400, speed: 2895},
}

func TestSpeed(t *testing.T) {
	logger.ConfigureDefaultLoggerFromEnv()

	for _, tc := range speedTestCases {
		actual := calculateSpeed(tc.mile1, tc.mile2, tc.timestamp1, tc.timestamp2)
		if actual != tc.speed {
			t.Errorf("Speed:\nexpected: %d\nactual: %d", tc.speed, actual)
		}
	}
}
