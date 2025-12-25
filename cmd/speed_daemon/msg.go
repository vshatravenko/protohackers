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

	timestamp := binary.BigEndian.Uint32(input[offset:])
	slog.Debug("Parsed plate message", "input", input, "plate", plate, "offset", offset, "timestamp", timestamp)
	offset += 4

	return &plateMsg{
		plate:     plate,
		timestamp: timestamp,
	}, offset
}

func (pm *plateMsg) Bytes() []byte {
	res := []byte{}
	res, _ = binary.Append(res, binary.BigEndian, msgTypes["plate"])
	plateBytes := fixedStrToBytes(pm.plate)
	res, _ = binary.Append(res, binary.BigEndian, plateBytes)
	res = binary.BigEndian.AppendUint32(res, pm.timestamp)

	return res
}

func (pm *plateMsg) equal(input *plateMsg) bool {
	plate := pm.plate == input.plate
	ts := pm.timestamp == input.timestamp

	slog.Info("plateMsg equal() results", "plate", plate, "ts", ts)
	return plate && ts
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
	input = input[1:] // msg type
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
	res := []byte{msgTypes["camera"]}
	buf := bytes.Buffer{}

	err := binary.Write(&buf, binary.BigEndian, cm)
	if err != nil {
		slog.Warn("error converting iAmCameraMsg to bytes", "err", err, "msg", cm)
	}

	return append(res, buf.Bytes()...)
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
	length := int(uint8(input[start]))

	if length == 0 {
		return ""
	}

	buf := input[start+1 : start+1+length]
	res := string(buf)

	return res
}

func fixedStrToBytes(input string) []byte {
	length := uint8(len(input))

	res := []byte{length}
	res, _ = binary.Append(res, binary.BigEndian, []byte(input))

	return res
}
