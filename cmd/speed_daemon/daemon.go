package main

import (
	"encoding/binary"
	"io"
	"log/slog"
	"math"
	"net"
	"slices"
	"time"
)

type daemon struct {
	cameras     map[uint16]*camera
	dispatchers []*dispatcher
	records     map[string][]record
	tickets     map[string][]*ticket
	heartbeat   chan *net.TCPConn
}

func newDaemon() *daemon {
	return &daemon{
		cameras:     make(map[uint16]*camera),
		dispatchers: make([]*dispatcher, 0),
		records:     make(map[string][]record),
		tickets:     make(map[string][]*ticket),
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
func (d *daemon) addCamera(conn net.Conn, road, mile, limit uint16) {
	slog.Info("handling new camera", "addr", conn.RemoteAddr(), "road", road, "mile", mile, "limit", limit)
	c := &camera{conn: conn, road: road, mile: mile, limit: limit}
	d.cameras[road] = c
	defer closeConn(conn)

	for {
		b := make([]byte, bufSize)
		n, err := c.conn.Read(b)
		if err != nil && err != io.EOF {
			slog.Warn("failed to read from camera conn", "road", c.road, "mile", c.mile, "err", err.Error())
			return
		}

		payload := b[:n]
		msg := parsePlateMsg(payload[1:])
		slog.Info("parsed plate message", "road", c.road, "mile", c.mile, "msg", msg)
	}
}

/*
Algo:
  - Look for existing records of input plate
  - If found, sort the array, and compare the latest timestamp with the current one,
    calculate the speed, if it exceeds the speed limit, issue a new ticket to the dispatcher,
    record the ticket to avoid issuing new ones
  - Otherwise, append the current record to said plate's array
*/
func (d *daemon) handlePlate(plate string, ts uint32, road, mile uint16, limit int) {
	if _, ok := d.records[plate]; ok {
		sortRecords := func(a, b record) int {
			return int(a.timestamp - b.timestamp)
		}
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

				speed := int(math.Round(distance / duration))

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

					d.tickets[plate] = append(d.tickets[plate], newTicket)
					go d.dispatchTicket(newTicket)
				}
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

func (t *ticket) Bytes() []byte {
	res := fixedStrToBytes(t.plate)

	res = binary.BigEndian.AppendUint32(res, t.timestamp1)
	res = binary.BigEndian.AppendUint16(res, t.mile1)
	res = binary.BigEndian.AppendUint32(res, t.timestamp2)
	res = binary.BigEndian.AppendUint16(res, t.mile2)
	res = binary.BigEndian.AppendUint16(res, t.road)
	res = binary.BigEndian.AppendUint16(res, t.speed)

	return res
}

func closeConn(conn net.Conn) {
	if err := conn.Close(); err != nil {
		slog.Info("could not close conn", "addr", conn.RemoteAddr(), "err", err.Error())
	}
}
