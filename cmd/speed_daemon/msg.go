package main

import (
	"encoding/binary"
	"fmt"
	"strings"
)

var msgTypes = map[string]uint8{
	"error":          0x10,
	"plate":          0x20,
	"ticket":         0x21,
	"want_heartbeat": 0x40,
	"heartbeat":      0x41,
	"camera":         0x80,
	"dispatcher":     0x81,
}

// I could've parsed each payload directly into the target struct
// but this way I could create mock payloads for testing

type errorMsg struct {
	msg string
}

func parseErrorMsg(input []byte) errorMsg {
	msg := parseStr(input, 1)

	return errorMsg{msg}
}

type plateMsg struct {
	plate     string
	timestamp uint32
}

func parsePlateMsg(input []byte) plateMsg {
	plate := parseStr(input, 0)
	offset := len(plate)

	timestamp := binary.BigEndian.Uint32(input[offset:])

	return plateMsg{
		plate:     plate,
		timestamp: timestamp,
	}
}

type ticketMsg struct {
	plate      string
	road       uint16
	mile1      uint16
	timestamp1 uint16
	mile2      uint16
	timestamp2 uint16
	speed      uint16 // 100x miles per hour
}

type wantHeartbeatMsg struct {
	interval uint32 // deciseconds, 25 == 2.5 seconds
}

func parseWantHeartbeatMsg(input []byte) wantHeartbeatMsg {
	interval := binary.BigEndian.Uint32(input)

	return wantHeartbeatMsg{interval: interval}
}

type heartbeatMsg uint8

type iAmCameraMsg struct {
	road  uint16
	mile  uint16
	limit uint16 // miles per hour
}

func parseCameraMsg(input []byte) iAmCameraMsg {
	road := binary.BigEndian.Uint16(input[:2])
	mile := binary.BigEndian.Uint16(input[2:4])
	limit := binary.BigEndian.Uint16(input[4:])

	return iAmCameraMsg{road: road, mile: mile, limit: limit}
}

type iAmDispatcherMsg struct {
	numRoads uint8
	roads    []uint16
}

func parseDispatcherMsg(input []byte) iAmDispatcherMsg {
	numRoads := uint8(input[0])

	roads := make([]uint16, numRoads)
	for i := uint8(0); i < numRoads; i++ {
		roads[i] = binary.BigEndian.Uint16(input[i : i+2])
	}

	return iAmDispatcherMsg{numRoads: numRoads, roads: roads}
}

func parseStr(input []byte, start int) string {
	length := uint8(input[0])

	if length == 0 {
		return ""
	}

	var b strings.Builder
	for i := uint8(start); i < length; i++ {
		fmt.Fprint(&b, input[i])
	}

	return b.String()
}

func fixedStrToBytes(input string) []byte {
	length := uint8(len(input))

	res := make([]byte, len(input)+1)
	res[0] = length
	for i := 1; i < len(res); i++ {
		res[i] = uint8(input[i])
	}

	return res
}
