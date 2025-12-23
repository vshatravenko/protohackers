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
	records     map[string][]record
	recordsLock *sync.RWMutex
	tickets     map[string][]*ticket
	ticketsLock *sync.RWMutex
	heartbeat   chan *net.TCPConn
}

func newDaemon() *daemon {
	return &daemon{
		dispatchers: make([]*dispatcher, 0),
		records:     make(map[string][]record),
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
	slog.Info("handling new camera", "addr", conn.RemoteAddr(), "payload", initPayload)
	cMsg := parseCameraMsg(initPayload)
	c := &camera{conn: conn, road: cMsg.road, mile: cMsg.mile, limit: cMsg.limit}
	slog.Info("parsed new camera", "addr", conn.RemoteAddr(), "camera", c)
	defer closeConn(conn)

	// the length of iAmCamera msg is 6
	initPayload = initPayload[6:]

	if len(initPayload) > 0 {
		d.processCameraPayload(c, initPayload)
	}

	for {
		b := make([]byte, bufSize)
		n, err := c.conn.Read(b)
		if err != nil && err != io.EOF {
			slog.Warn("failed to read from camera conn", "road", c.road, "mile", c.mile, "err", err.Error())
			return
		}

		payload := b[:n]
		d.processCameraPayload(c, payload)
	}
}

func (d *daemon) processCameraPayload(c *camera, payload []byte) error {
	for len(payload) > 0 {
		slog.Info("processing camera payload", "addr", c.conn.RemoteAddr())
		switch payload[0] {
		case msgTypes["want_heartbeat"]:
			slog.Info("parsing camera heartbeat msg", "addr", c.conn.RemoteAddr())
			msg := parseWantHeartbeatMsg(payload[1:])
			slog.Debug("will send out a heartbeat", "addr", c.conn.RemoteAddr(), "msg", msg)
			payload = payload[5:] // 1 - msg type + 4 - want_heartbeat
		case msgTypes["plate"]:
			slog.Info("parsing camera plate msg", "addr", c.conn.RemoteAddr())
			msg, offset := parsePlateMsg(payload[1:])
			slog.Info("parsed plate message", "road", c.road, "mile", c.mile, "msg", msg)
			d.handlePlate(msg.plate, msg.timestamp, c.road, c.mile, c.limit)
			payload = payload[offset+1:]
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

	if _, ok := d.records[plate]; ok {
		slices.SortFunc(d.records[plate], sortRecords)

		records := d.records[plate]
		for i := 0; i < len(d.records[plate]); i++ {
			if records[i].timestamp == ts {
				if i == 0 {
					return
				}

				prevRecord := records[i-1]

				duration := float64(ts - prevRecord.timestamp)
				distance := float64(mile - prevRecord.mile)

				speed := uint16(math.Round(distance/duration)) * 100 // as per the specification

				if speed > limit {
					newTicket := &ticket{
						plate:      plate,
						timestamp1: prevRecord.timestamp,
						mile1:      prevRecord.mile,
						timestamp2: ts,
						mile2:      mile,
						road:       road,
						sent:       false,
					}

					// TODO: check the latest issued ticket for that day

					slog.Debug("Issued a new ticket", "ticket", newTicket)
					d.ticketsLock.Lock()
					d.tickets[plate] = append(d.tickets[plate], newTicket)
					d.ticketsLock.Unlock()
					go d.dispatchTicket(newTicket)
				}

				// Should we delete the previous timestamp and keep only the latest one?
			}
		}
	} else {
		newRecord := record{plate: plate, mile: mile, timestamp: ts}
		d.records[plate] = append(d.records[plate], newRecord)
	}
}

// TODO: handle cases when the target dispatcher is not available
func (d *daemon) dispatchTicket(t *ticket) {
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

func sortRecords(a, b record) int {
	return int(a.timestamp - b.timestamp)
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
	slog.Debug("parseTicket: start", "input", input)
	offset := 1
	plate := parseFixedStr(input, offset)
	offset += len(plate) + 1

	road := binary.BigEndian.Uint16(input[offset : offset+2])
	slog.Debug("parseTicket: road", "input", input[offset:offset+2], "parsed", road)
	offset += 2

	mile1 := binary.BigEndian.Uint16(input[offset : offset+2])
	slog.Debug("parseTicket: mile1", "input", input[offset:offset+2], "parsed", mile1)
	offset += 2

	ts1 := binary.BigEndian.Uint32(input[offset : offset+4])
	slog.Debug("parseTicket: ts1", "input", input[offset:offset+4], "parsed", ts1)
	offset += 4

	mile2 := binary.BigEndian.Uint16(input[offset : offset+2])
	slog.Debug("parseTicket: mile2", "input", input[offset:offset+2], "parsed", mile2)
	offset += 2

	ts2 := binary.BigEndian.Uint32(input[offset : offset+4])
	slog.Debug("parseTicket: ts2", "input", input[offset:offset+4], "parsed", ts2)
	offset += 4

	speed := binary.BigEndian.Uint16(input[offset : offset+2])
	slog.Debug("parseTicket: speed", "input", input[offset:offset+2], "parsed", speed)

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
