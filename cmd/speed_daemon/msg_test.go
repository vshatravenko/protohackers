package main

import (
	"testing"

	"github.com/vshatravenko/protohackers/internal/logger"
)

var (
	cameraMsg = iAmCameraMsg{
		road:  123,
		mile:  8,
		limit: 60,
	}
	dispatherMsg = iAmDispatcherMsg{
		numRoads: 2,
		roads:    []uint16{123, 124},
	}
	plate = plateMsg{
		plate:     "UN1X",
		timestamp: 45,
	}
	plateOffset    = 10 // 1 msg type byte + 1 length byte + 4 len("UN1X") + 4 timestamp uint32
	expectedTicket = ticket{
		plate:      "UN1X",
		road:       123,
		mile1:      8,
		timestamp1: 0,
		mile2:      9,
		timestamp2: 45,
		speed:      8000,
	}
	whm = wantHeartbeatMsg{
		interval: 25,
	}
)

func TestMsgEncodingParsing(t *testing.T) {
	logger.ConfigureDefaultLoggerFromEnv()

	b := cameraMsg.Bytes()
	parsedCM := parseCameraMsg(b)

	if !cameraMsg.equal(parsedCM) {
		t.Errorf("iAmCameraMsg:\nexpected: %+v\nactual: %+v", cameraMsg, *parsedCM)
	}

	b = dispatherMsg.Bytes()
	parsedDM := parseDispatcherMsg(b)
	if !dispatherMsg.equal(parsedDM) {
		t.Errorf("iAmDispatcherMsg:\nexpected: %+v\nactual: %+v", dispatherMsg, *parsedDM)
	}

	b = plate.Bytes()
	parsedPlate, parsedOffset := parsePlateMsg(b)
	if !plate.equal(parsedPlate) {
		t.Errorf("plateMsg:\nexpected: %+v\nactual: %+v", plate, *parsedPlate)
	}

	if parsedOffset != plateOffset {
		t.Errorf("plateMsg offset:\nexpected: %d\nactual: %d", plateOffset, parsedOffset)
	}

	b = expectedTicket.Bytes()
	parsedTicket := parseTicket(b)
	if !expectedTicket.equal(parsedTicket) {
		t.Errorf("ticket:\nexpected: %+v\nactual: %+v", expectedTicket, *parsedTicket)
	}

	b = whm.Bytes()
	parsedWHM := parseWantHeartbeatMsg(b)
	if !whm.equal(parsedWHM) {
		t.Errorf("want_heartbeat:\nexpected: %+v\nactual: %+v", whm, *parsedWHM)
	}

}
