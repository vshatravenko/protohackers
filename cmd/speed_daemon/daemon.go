package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"slices"
	"sync"
	"time"
)

type daemon struct {
	dispatchers []*dispatcher
	records     map[string][]*record
	recordsLock *sync.RWMutex
	tickets     map[string][]*ticket
	ticketsLock *sync.RWMutex
	heartbeat   chan *net.TCPConn
}

func newDaemon() *daemon {
	return &daemon{
		dispatchers: make([]*dispatcher, 0),
		records:     make(map[string][]*record),
		recordsLock: new(sync.RWMutex),
		tickets:     make(map[string][]*ticket),
		ticketsLock: new(sync.RWMutex),
	}
}

func (d *daemon) handleHeartbeat() {
	for {
		select {
		case conn := <-d.heartbeat:
			msg := new(heartbeatMsg)
			_, err := conn.Write(msg.Bytes())
			if err != nil {
				slog.Warn("failed to send a heartbeat to conn", "addr", conn.RemoteAddr(), "err", err.Error())
			}
		default:
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// Should we add a camera and create new goroutines tracked by a waitgroup
// or simply use the existing process to keep the connection loop?
func (d *daemon) handleCamera(conn net.Conn, initPayload []byte) {
	slog.Info("handling new camera", "addr", conn.RemoteAddr())
	cMsg := parseCameraMsg(initPayload)
	c := &camera{conn: conn, road: cMsg.road, mile: cMsg.mile, limit: cMsg.limit}
	slog.Info("parsed new camera", "addr", conn.RemoteAddr(), "camera", c)
	defer closeConn(conn)

	// the length of iAmCamera msg is 7(msg type byte + 3 uint16 fields)
	initPayload = initPayload[7:]

	if len(initPayload) > 0 {
		slog.Debug("handling init camera payload", "addr", conn.RemoteAddr())
		if err := d.processCameraPayload(c, initPayload); err != nil {
			slog.Warn("failed to process init camera payload", "addr", conn.RemoteAddr(), "err", err.Error())
			return
		}
	}

	slog.Debug("waiting for a new camera payload", "addr", conn.RemoteAddr())
	for {
		b := make([]byte, bufSize)
		n, err := c.conn.Read(b)
		if err != nil && err != io.EOF {
			slog.Warn("failed to read from camera conn", "road", c.road, "mile", c.mile, "err", err.Error())
			return
		}

		payload := b[:n]
		if err := d.processCameraPayload(c, payload); err != nil {
			slog.Warn("failed to process camera payload", "addr", conn.RemoteAddr(), "err", err.Error())
			return
		}
	}
}

func (d *daemon) processCameraPayload(c *camera, payload []byte) error {
	for len(payload) > 0 {
		slog.Info("processing camera payload", "addr", c.conn.RemoteAddr(), "length", len(payload))
		switch payload[0] {
		case msgTypes["want_heartbeat"]:
			slog.Info("parsing camera heartbeat msg", "addr", c.conn.RemoteAddr())
			msg := parseWantHeartbeatMsg(payload)
			slog.Debug("will send out a heartbeat", "addr", c.conn.RemoteAddr(), "msg", msg)
			payload = payload[5:] // 1 - msg type + 4 - want_heartbeat
		case msgTypes["plate"]:
			slog.Info("parsing camera plate msg", "addr", c.conn.RemoteAddr())
			msg, offset := parsePlateMsg(payload)
			slog.Info("parsed plate message", "road", c.road, "mile", c.mile, "msg", msg)
			d.handlePlate(msg.plate, msg.timestamp, c.road, c.mile, c.limit)
			payload = payload[offset:]
		default:
			slog.Info("invalid payload type", "addr", c.conn.RemoteAddr())
			return fmt.Errorf("%b message type is invalid", payload[0])
		}
	}
	return nil
}

/*
Algo:
  - Look for existing records of input plate
  - If found, sort the array, and compare the latest timestamp with the current one,
    calculate the speed, if it exceeds the speed limit, issue a new ticket to the dispatcher,
    record the ticket to avoid issuing new ones
  - Otherwise, append the current record to said plate's array
*/
func (d *daemon) handlePlate(plate string, ts uint32, road, mile, limit uint16) {
	d.recordsLock.Lock()
	defer d.recordsLock.Unlock()

	_, exists := d.records[plate]

	curRecord := &record{plate: plate, mile: mile, timestamp: ts}
	slog.Debug("inserting record for plate", "plate", plate, "record", curRecord)
	d.records[plate] = append(d.records[plate], curRecord)

	if exists {
		slog.Debug("found existing records, checking for speed limit", "plate", plate)
		records := filterRecordsByPlate(d.records[plate], plate)
		slices.SortFunc(records, sortRecords)

		for i := range len(records) {
			var prevRecord *record
			if records[i].timestamp == ts {
				// TODO: we may need to retroactively check for speed violations
				// in case earlier tickets come in the future
				if i == 0 {
					slog.Debug("this is the earliest record, comparing to the future one", "plate", plate, "timestamp", ts)
					prevRecord = curRecord
					curRecord = records[i+1]
				} else {
					prevRecord = records[i-1]
					slog.Debug("found previous chronological record", "plate", plate, "record", *prevRecord)
				}

				speed := calculateSpeed(prevRecord.mile, curRecord.mile, prevRecord.timestamp, curRecord.timestamp)
				slog.Debug("calculated speed",
					"mile1", prevRecord.mile, "mile2", curRecord.mile,
					"ts1", prevRecord.timestamp, "ts2", curRecord.timestamp,
					"speed", speed,
				)

				mph := uint16(math.Round(float64(speed) / 100))
				slog.Info("converted speed to mph", "speed", speed, "mph", mph)

				if mph > limit {
					newTicket := &ticket{
						plate:      plate,
						timestamp1: prevRecord.timestamp,
						mile1:      prevRecord.mile,
						timestamp2: curRecord.timestamp,
						mile2:      curRecord.mile,
						road:       road,
						speed:      speed,
						sent:       false,
					}
					slog.Debug("detected speed violation", "speed", speed, "mph", mph, "limit", limit)

					// TODO: check the latest issued ticket for that day

					slog.Debug("Issued a new ticket", "ticket", newTicket)
					d.ticketsLock.Lock()
					d.tickets[plate] = append(d.tickets[plate], newTicket)
					d.ticketsLock.Unlock()
					// TODO: put this into a channel that can be re-enqueued after a timeout
					// if there are no dispatchers available or if the transfer fails
					go d.dispatchTicket(newTicket)
				}
				// Should we delete the previous timestamp and keep only the latest one?
			}
		}
	}
}

// TODO: handle cases when the target dispatcher is not available
func (d *daemon) dispatchTicket(t *ticket) {
	slog.Debug("looking for dispatchers", "timestamp", t.timestamp2)
	for _, disp := range d.dispatchers {
		for _, r := range disp.roads {
			if r == t.road {
				payload := t.Bytes()
				_, err := disp.conn.Write(payload)
				if err == nil {
					t.sent = true
				} else {
					slog.Warn("failed to dispatch ticket", "addr", disp.conn.RemoteAddr(), "err", err.Error(), "ticket", t)
				}
			}
		}
	}
}

func (d *daemon) addDispatcher(conn net.Conn, roads []uint16) {
	slog.Info("handling new dispatcher", "addr", conn.RemoteAddr(), "roads", roads)
	disp := newDispatcher(conn, roads)
	d.dispatchers = append(d.dispatchers, disp)
	go disp.watch()

	for {
		b := make([]byte, bufSize)
		_, err := disp.conn.Read(b)
		if err != nil && err != io.EOF {
			slog.Warn("failed to read from disp conn", "addr", disp.conn.RemoteAddr, "err", err.Error())
			disp.kill <- 1
			return
		}
	}
}

type camera struct {
	conn  net.Conn
	road  uint16
	mile  uint16
	limit uint16
}

type dispatcher struct {
	conn  net.Conn
	roads []uint16
	comm  chan *ticket
	kill  chan int
}

func newDispatcher(conn net.Conn, roads []uint16) *dispatcher {
	return &dispatcher{
		conn:  conn,
		roads: roads,
		comm:  make(chan *ticket, 10),
		kill:  make(chan int),
	}
}

func (disp *dispatcher) watch() {
	for {
		select {
		case t := <-disp.comm:
			slog.Debug("received a ticket", "addr", disp.conn.RemoteAddr(), "ticket", t)
		case <-disp.kill:
			return
		default:
			time.Sleep(500 * time.Millisecond)
		}
	}
}

type record struct {
	plate     string
	mile      uint16
	timestamp uint32
}

func sortRecords(a, b *record) int {
	if a.timestamp > b.timestamp {
		return 1
	} else if a.timestamp < b.timestamp {
		return -1
	} else {
		return 0
	}
}

type ticket struct {
	plate      string
	timestamp1 uint32
	mile1      uint16
	timestamp2 uint32
	mile2      uint16
	road       uint16
	speed      uint16
	sent       bool
}

func parseTicket(input []byte) *ticket {
	offset := 1
	plate := parseFixedStr(input, offset)
	offset += len(plate) + 1

	road := binary.BigEndian.Uint16(input[offset : offset+2])
	offset += 2

	mile1 := binary.BigEndian.Uint16(input[offset : offset+2])
	offset += 2

	ts1 := binary.BigEndian.Uint32(input[offset : offset+4])
	offset += 4

	mile2 := binary.BigEndian.Uint16(input[offset : offset+2])
	offset += 2

	ts2 := binary.BigEndian.Uint32(input[offset : offset+4])
	offset += 4

	speed := binary.BigEndian.Uint16(input[offset : offset+2])

	return &ticket{
		plate:      plate,
		road:       road,
		mile1:      mile1,
		timestamp1: ts1,
		mile2:      mile2,
		timestamp2: ts2,
		speed:      speed,
	}
}

func (t *ticket) equal(input *ticket) bool {
	plate := t.plate == input.plate
	road := t.road == input.road
	mile1 := t.mile1 == input.mile1
	ts1 := t.timestamp1 == input.timestamp1
	mile2 := t.mile2 == input.mile2
	ts2 := t.timestamp2 == input.timestamp2
	speed := t.speed == input.speed

	slog.Info("ticket equal() results", "plate", plate,
		"road", road, "mile1", mile1, "ts1", ts1, "mile2", mile2,
		"ts2", ts2, "speed", speed)
	return plate && road && mile1 && ts1 && mile2 && ts2 && speed
}

func (t *ticket) Bytes() []byte {
	res := []byte{msgTypes["ticket"]}
	res, _ = binary.Append(res, binary.BigEndian, fixedStrToBytes(t.plate))

	res = binary.BigEndian.AppendUint16(res, t.road)
	res = binary.BigEndian.AppendUint16(res, t.mile1)
	res = binary.BigEndian.AppendUint32(res, t.timestamp1)
	res = binary.BigEndian.AppendUint16(res, t.mile2)
	res = binary.BigEndian.AppendUint32(res, t.timestamp2)
	res = binary.BigEndian.AppendUint16(res, t.speed)

	return res
}

func closeConn(conn net.Conn) {
	if err := conn.Close(); err != nil {
		slog.Info("could not close conn", "addr", conn.RemoteAddr(), "err", err.Error())
	}
}

func filterRecordsByPlate(records []*record, plate string) []*record {
	res := []*record{}

	for _, rec := range records {
		if rec.plate == plate {
			res = append(res, rec)
		}
	}

	return res
}

func calculateSpeed(mile1, mile2 uint16, ts1, ts2 uint32) uint16 {
	duration := float64(ts2 - ts1)
	distance := float64(mile2 - mile1)
	mph := distance * 3600 / duration
	speed := uint16(mph * 100) // 100x as per the specification

	return speed
}

func prettyPrintRecords(records []*record) string {
	res := ""
	for _, r := range records {
		res += fmt.Sprintf("%+v ", *r)
	}
	return res
}
