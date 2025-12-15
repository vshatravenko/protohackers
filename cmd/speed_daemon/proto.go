package main

import (
	"fmt"
	"log/slog"
	"strings"
)

type errorMsg struct {
	msg string
}

func newErrorMsg(input []byte) errorMsg {
	msg := parseStr(input, 1)

	return errorMsg{msg}
}

type plateMsg struct {
	plate     string
	timestamp uint32
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
	interval uint32
}

type heartbeatMsg uint8

type iAmCameraMsg struct {
	road  uint16
	mile  uint16
	limit uint16 // miles per hour
}

type iAmDispatcher struct {
	numRoads uint8
	roads    []uint16
}

func handlePayload(payload []byte) {
	msgType := uint8(payload[0])
	payload = payload[1:]

	switch msgType {
	case 0x10:
		newErrorMsg(payload)
	case 0x20:
		slog.Debug("parsing plate msg")
	case 0x21:
		slog.Debug("parsing ticket msg")
	case 0x40:
		slog.Debug("parsing want heartbeat msg")
	case 0x41:
		slog.Debug("parsing heartbeat msg")
	case 0x80:
		slog.Debug("parsing iAmCamera msg")
	case 0x81:
		slog.Debug("parsing iAmDispatcher msg")
	}

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
