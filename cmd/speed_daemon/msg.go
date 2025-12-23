package main

import (
	"bytes"
	"encoding/binary"
	"log/slog"
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

func parseErrorMsg(input []byte) *errorMsg {
	msg := parseFixedStr(input, 1)

	return &errorMsg{msg}
}

type plateMsg struct {
	plate     string
	timestamp uint32
}

func parsePlateMsg(input []byte) (*plateMsg, int) {
	offset := 1 // msg type byte
	plate := parseFixedStr(input, offset)
	offset += len(plate) + 1

	slog.Debug("Parsed plate message", "input", input, "plate", plate, "offset", offset, "remainder", input[offset:])
	timestamp := binary.BigEndian.Uint32(input[offset:])
	offset += 4

	return &plateMsg{
		plate:     plate,
		timestamp: timestamp,
	}, offset
}

func (pm *plateMsg) Bytes() []byte {
	res := []byte{}
	slog.Debug("plateMsg.Bytes() start", "res", res)
	res, _ = binary.Append(res, binary.BigEndian, msgTypes["plate"])
	slog.Debug("plateMsg.Bytes() type", "res", res)
	plateBytes := fixedStrToBytes(pm.plate)
	res, _ = binary.Append(res, binary.BigEndian, plateBytes)
	slog.Debug("plateMsg.Bytes() plate number", "res", res)
	res = binary.BigEndian.AppendUint32(res, pm.timestamp)
	slog.Debug("plateMsg.Bytes() ts", "res", res, "ts", pm.timestamp)

	return res
}

func (pm *plateMsg) equal(input *plateMsg) bool {
	plate := pm.plate == input.plate
	ts := pm.timestamp == input.timestamp

	slog.Info("plateMsg equal results", "plate", plate, "ts", ts)
	return plate && ts
}

type ticketMsg struct {
	plate      string
	road       uint16
	mile1      uint16
	timestamp1 uint32
	mile2      uint16
	timestamp2 uint32
	speed      uint16 // 100x miles per hour
}

func parseTicketMsg(input []byte) *ticketMsg {
	plate := parseFixedStr(input, 0)
	offset := len(plate)

	road := binary.BigEndian.Uint16(input[offset : offset+2])
	offset += 2

	mile1 := binary.BigEndian.Uint16(input[offset : offset+2])
	offset += 2

	ts1 := binary.BigEndian.Uint32(input[offset : offset+2])
	offset += 4

	mile2 := binary.BigEndian.Uint16(input[offset : offset+2])
	offset += 2

	ts2 := binary.BigEndian.Uint32(input[offset : offset+2])
	offset += 4

	speed := binary.BigEndian.Uint16(input[offset : offset+2])

	return &ticketMsg{
		plate:      plate,
		road:       road,
		mile1:      mile1,
		timestamp1: ts1,
		mile2:      mile2,
		timestamp2: ts2,
		speed:      speed,
	}
}

func (tm *ticketMsg) equal(input *ticketMsg) bool {
	plate := tm.plate == input.plate
	road := tm.road == input.road
	mile1 := tm.mile1 == input.mile1
	ts1 := tm.timestamp1 == input.timestamp1
	mile2 := tm.mile2 == input.mile2
	ts2 := tm.timestamp2 == input.timestamp2
	speed := tm.speed == input.speed

	slog.Info("ticketMsg equal results", "plate", plate,
		"road", road, "mile1", mile1, "ts1", ts1, "mile2", mile2,
		"ts2", ts2, "speed", speed)
	return plate && road && mile1 && ts1 && mile2 && ts2 && speed
}

type wantHeartbeatMsg struct {
	interval uint32 // deciseconds, 25 == 2.5 seconds
}

func (whm *wantHeartbeatMsg) Bytes() []byte {
	res := []byte{msgTypes["want_heartbeat"]}
	binary.BigEndian.AppendUint32(res, whm.interval)

	return res
}

func parseWantHeartbeatMsg(input []byte) wantHeartbeatMsg {
	interval := binary.BigEndian.Uint32(input)

	return wantHeartbeatMsg{interval: interval}
}

type heartbeatMsg uint8

func (hm *heartbeatMsg) Bytes() []byte {
	return []byte{msgTypes["heartbeat"]}
}

type iAmCameraMsg struct {
	road  uint16
	mile  uint16
	limit uint16 // miles per hour
}

func parseCameraMsg(input []byte) *iAmCameraMsg {
	road := binary.BigEndian.Uint16(input[:2])
	mile := binary.BigEndian.Uint16(input[2:4])
	limit := binary.BigEndian.Uint16(input[4:])

	return &iAmCameraMsg{road: road, mile: mile, limit: limit}
}

func (cm *iAmCameraMsg) equal(input *iAmCameraMsg) bool {
	mile := cm.mile == input.mile
	road := cm.road == input.road
	limit := cm.limit == input.limit

	slog.Debug("iAmCameraMsg equal results", "mile", mile, "road", road, "limit", limit)
	return mile && road && limit
}

func (cm *iAmCameraMsg) Bytes() []byte {
	buf := bytes.Buffer{}

	err := binary.Write(&buf, binary.BigEndian, cm)
	if err != nil {
		slog.Warn("error converting iAmCameraMsg to bytes", "err", err, "msg", cm)
	}

	return buf.Bytes()
}

type iAmDispatcherMsg struct {
	numRoads uint8
	roads    []uint16
}

func parseDispatcherMsg(input []byte) *iAmDispatcherMsg {
	offset := 1 // strip the msg type
	numRoads := uint8(input[offset])
	slog.Debug("parseDispatcherMsg parsing", "numRoads", numRoads)
	offset++

	roads := make([]uint16, numRoads)
	for i := range int(numRoads) {
		roads[i] = binary.BigEndian.Uint16(input[offset : offset+2])
		offset += 2
	}

	return &iAmDispatcherMsg{numRoads: numRoads, roads: roads}
}

func (cm *iAmDispatcherMsg) equal(input *iAmDispatcherMsg) bool {
	numRoads := cm.numRoads == input.numRoads
	roads := true

	for i := range input.roads {
		if cm.roads[i] != input.roads[i] {
			roads = false
			break
		}
	}

	slog.Debug("iAmDispatcherMsg equal results", "numRoads", numRoads, "roads", roads)
	return numRoads && roads
}

func (dm *iAmDispatcherMsg) Bytes() []byte {
	res := []byte{}
	res, err := binary.Append(res, binary.BigEndian, msgTypes["dispatcher"])
	if err != nil {
		slog.Warn("iAmDispatherMsg Bytes()", "err", err, "msg", dm)
	}

	res, err = binary.Append(res, binary.BigEndian, dm.numRoads)
	if err != nil {
		slog.Warn("iAmDispatherMsg Bytes()", "err", err, "msg", dm)
	}

	for _, road := range dm.roads {
		res, err = binary.Append(res, binary.BigEndian, road)
		if err != nil {
			slog.Warn("iAmDispatherMsg Bytes()", "err", err, "msg", dm)
		}
	}

	return res
}

func parseFixedStr(input []byte, start int) string {
	slog.Debug("parseFixedStr started", "start", start, "input", input, "start", start, "effective", input[start:])
	length := int(uint8(input[start]))
	slog.Debug("parseFixedStr detected length", "length", length)

	if length == 0 {
		return ""
	}

	buf := input[start+1 : start+1+length]
	res := string(buf)
	slog.Debug("parseFixedStr string-based", "res", res)

	//	var b strings.Builder
	//	for i := start; i < length; i++ {
	//		fmt.Fprint(&b, input[start+i])
	//	}
	//	slog.Debug("parseFixedStr parsed str", "input", input[start:], "res", b.String())

	return res
}

func fixedStrToBytes(input string) []byte {
	length := uint8(len(input))

	res := []byte{length}
	slog.Debug("fixedStrToBytes() pre", "input", input, "res", res, "length", length)
	res, _ = binary.Append(res, binary.BigEndian, []byte(input))
	slog.Debug("fixedStrToBytes() post", "input", input, "res", res, "length", length)

	return res
}
