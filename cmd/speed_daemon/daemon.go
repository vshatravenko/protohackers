package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	ticketsToSendSize       = 20
	ticketsToSendIntervalMS = 500
)

var (
	errExistingHeartbeat = errors.New("there is an existing heartbeat")
	errIncorrectMsgType  = errors.New("incorrect message type")
)

type daemon struct {
	dispatchers   []*dispatcher
	dispLock      *sync.RWMutex
	records       map[string][]*record
	recordsLock   *sync.RWMutex
	tickets       map[string]map[string]*ticket
	ticketsLock   *sync.RWMutex
	ticketsToSend chan *ticket
	heartbeats    map[*net.TCPConn]chan int
	hbLock        *sync.RWMutex
}

func newDaemon() *daemon {
	return &daemon{
		dispatchers:   make([]*dispatcher, 0),
		dispLock:      new(sync.RWMutex),
		records:       make(map[string][]*record),
		recordsLock:   new(sync.RWMutex),
		tickets:       make(map[string]map[string]*ticket),
		ticketsLock:   new(sync.RWMutex),
		ticketsToSend: make(chan *ticket, ticketsToSendSize),
		heartbeats:    make(map[*net.TCPConn]chan int),
		hbLock:        new(sync.RWMutex),
	}
}

func (d *daemon) handleConn(c net.Conn, payload []byte) {
	msgType := uint8(payload[0])

	conn := c.(*net.TCPConn)

	switch msgType {
	case msgTypes["camera"]:
		slog.Debug("parsed camera msg", "addr", conn.RemoteAddr())
		d.handleCamera(conn, payload)
	case msgTypes["dispatcher"]:
		msg, err := parseDispatcherMsg(payload)
		if err != nil {
			errMsg := &errorMsg{msg: err.Error()}
			_, _ = conn.Write(errMsg.Bytes())
			return
		}
		slog.Debug("parsed dispatcher msg", "addr", conn.RemoteAddr(), "msg", msg)
		d.handleDispatcher(conn, msg.roads)
	case msgTypes["want_heartbeat"]:
		if err := d.handleWantHeartbeat(conn, payload); err != nil {
			slog.Warn("failed to handle want_heartbeat, closing conn", "addr", conn.RemoteAddr())
			sendErrMsg(conn, err.Error())
			_ = conn.Close()
		}
		offset := 5 // msg type (1 byte) + interval (uint32 - 4 bytes)
		if len(payload) > offset {
			d.handleConn(conn, payload[offset:])
		} else {
			buf := make([]byte, bufSize)
			n, err := conn.Read(buf)
			if err != nil && err != io.EOF {
				slog.Warn("failed to read from conn after want_heartbeat", "addr", conn.RemoteAddr(), "err", err.Error())
			}
			d.handleConn(conn, buf[:n])
		}
	default:
		slog.Info("received unexpected message type", "type", fmt.Sprintf("0x%X", msgType))
		sendErrMsg(conn, errIncorrectMsgType.Error())
		_ = conn.Close()
	}
}

func (d *daemon) handleWantHeartbeat(conn *net.TCPConn, payload []byte) error {
	msg, err := parseWantHeartbeatMsg(payload)
	if err != nil {
		return err
	}
	interval := time.Duration(msg.interval) * time.Millisecond * 100

	d.hbLock.Lock()
	defer d.hbLock.Unlock()

	if _, ok := d.heartbeats[conn]; ok {
		slog.Debug("detected conn with existing heartbeat, aborting", "addr", conn.RemoteAddr(), "heartbeats", d.heartbeats)
		return errExistingHeartbeat
	}
	d.heartbeats[conn] = make(chan int)

	slog.Debug("will start sending out heartbeats", "addr", conn.RemoteAddr(), "interval", interval)
	if interval == 0 {
		slog.Debug("detected a zero-interval want_heartbeat", "addr", conn.RemoteAddr())
	} else {
		go sendHeartbeat(conn, interval, d.heartbeats[conn])
	}

	return nil
}

func sendHeartbeat(conn *net.TCPConn, interval time.Duration, done chan int) {
	for {
		select {
		case <-done:
			slog.Debug("finished sending heartbeats")
			return
		case <-time.After(interval):
			msg := new(heartbeatMsg)
			_, err := conn.Write(msg.Bytes())
			if err != nil {
				slog.Warn("failed to send a heartbeat to conn", "addr", conn.RemoteAddr(), "err", err.Error())
				return
			}
		}
	}
}

func (d *daemon) handleCamera(conn *net.TCPConn, initPayload []byte) {
	slog.Info("handling new camera", "addr", conn.RemoteAddr())
	cMsg, err := parseCameraMsg(initPayload)
	if err != nil {
		errMsg := &errorMsg{msg: err.Error()}
		_, _ = conn.Write(errMsg.Bytes())
		return
	}
	c := &camera{conn: conn, road: cMsg.road, mile: cMsg.mile, limit: cMsg.limit}
	slog.Info("parsed new camera", "addr", conn.RemoteAddr(), "camera", c)
	defer closeConn(conn)
	defer d.stopHeartbeat(conn)

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
			errMsg := &errorMsg{msg: err.Error()}
			_, _ = c.conn.Write(errMsg.Bytes())
			return
		}

		payload := b[:n]
		if err := d.processCameraPayload(c, payload); err != nil {
			slog.Warn("failed to process camera payload", "addr", conn.RemoteAddr(), "err", err.Error())
			errMsg := &errorMsg{msg: err.Error()}
			_, _ = c.conn.Write(errMsg.Bytes())
			return
		}
	}
}

func (d *daemon) processCameraPayload(c *camera, payload []byte) error {
	for len(payload) > 0 {
		slog.Info("processing camera payload", "addr", c.conn.RemoteAddr(), "length", len(payload))
		switch payload[0] {
		case msgTypes["want_heartbeat"]:
			slog.Info("handling camera heartbeat msg", "addr", c.conn.RemoteAddr())
			if err := d.handleWantHeartbeat(c.conn, payload); err != nil {
				return err
			}
			payload = payload[5:] // msg type byte + uint32 (4 bytes)
		case msgTypes["plate"]:
			slog.Info("parsing camera plate msg", "addr", c.conn.RemoteAddr())
			msg, offset, err := parsePlateMsg(payload)
			if err != nil {
				return err
			}
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

	_, exists := d.records[plate]

	curRecord := &record{plate: plate, mile: mile, timestamp: ts}
	slog.Debug("inserting record for plate", "plate", plate, "record", curRecord)
	d.records[plate] = append(d.records[plate], curRecord)
	d.recordsLock.Unlock()

	date := time.Unix(int64(curRecord.timestamp), 0).Format("02-01-2006") // FYI, we have to use these in Go instead of YYYY...
	d.ticketsLock.RLock()
	if _, ok := d.tickets[plate][date]; ok {
		slog.Debug("found existing ticket for the current day, skipping speed limit check", "plate", plate, "date", date)
		return
	}
	d.ticketsLock.RUnlock()

	if exists {
		slog.Debug("found existing records, checking for speed limit", "plate", plate)
		d.recordsLock.RLock()
		defer d.recordsLock.RUnlock()
		records := filterRecordsByPlate(d.records[plate], plate)
		slices.SortFunc(records, sortRecords)

		var curPos int
		for i := range len(records) {
			if records[i].timestamp == curRecord.timestamp {
				curPos = i
				break
			}
		}

		// TODO: we need to ensure that the previous record doesn't have the same mile
		var prevRecord *record
		// TODO: we may need to retroactively check for speed violations
		// in case earlier tickets come in the future
		if curPos == 0 {
			slog.Debug("this is the earliest record, comparing to the future one", "plate", plate, "timestamp", ts)
			prevRecord = curRecord
			curRecord = records[curPos+1]
		} else {
			prevRecord = records[curPos-1]
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
			if _, ok := d.tickets[plate]; !ok {
				d.tickets[plate] = make(map[string]*ticket)

			}
			d.tickets[plate][date] = newTicket
			d.ticketsLock.Unlock()

			d.ticketsToSend <- newTicket
			// Should we delete the previous timestamp and keep only the latest one?
		}
	}
}

func (d *daemon) processPendingTickets() {
	for {
		select {
		case t := <-d.ticketsToSend:
			slog.Debug("processing pending ticket", "plate", t.plate, "road", t.road, "timestamp", t.timestamp2)
			go d.dispatchTicket(t)
		default:
			time.Sleep(ticketsToSendIntervalMS * time.Millisecond)
		}
	}
}

func (d *daemon) dispatchTicket(t *ticket) {
	slog.Debug("looking for dispatchers", "plate", t.plate, "road", t.road, "timestamp", t.timestamp2)
	for _, disp := range d.dispatchers {
		if slices.Contains(disp.roads, t.road) {
			payload := t.Bytes()
			if _, err := disp.conn.Write(payload); err != nil {
				slog.Warn("failed to dispatch ticket", "addr", disp.conn.RemoteAddr(), "err", err.Error(), "ticket", t)
				d.ticketsToSend <- t
			} else {
				t.sent = true
			}

			return
		}
	}
	slog.Debug("no dispatchers present, retrying", "plate", t.plate, "road", t.road, "timestamp", t.timestamp2)
	time.Sleep(ticketsToSendIntervalMS * time.Millisecond)
	d.ticketsToSend <- t
}

func (d *daemon) handleDispatcher(conn *net.TCPConn, roads []uint16) {
	slog.Info("handling new dispatcher", "addr", conn.RemoteAddr(), "roads", roads)
	disp := newDispatcher(conn, roads)
	d.dispatchers = append(d.dispatchers, disp)
	go disp.watch()
	defer d.removeDispatcher(disp)
	defer d.stopHeartbeat(disp.conn)

	for {
		b := make([]byte, bufSize)
		n, err := disp.conn.Read(b)
		if err != nil && err != io.EOF {
			slog.Warn("failed to read from disp conn", "addr", disp.conn.RemoteAddr, "err", err.Error())
			return
		}

		if n == 0 {
			slog.Warn("received a zero read from disp conn", "addr", disp.conn.RemoteAddr)
			return
		}

		payload := b[:n]
		msgType := payload[0]
		if msgType != msgTypes["want_heartbeat"] {
			slog.Warn("unexpected msgType from dispatcher, aborting", "addr", disp.conn.RemoteAddr(), "type", msgType)
			return
		}

		slog.Info("handling dispatcher heartbeat msg", "addr", disp.conn.RemoteAddr())
		if err := d.handleWantHeartbeat(disp.conn, payload); err != nil {
			errMsg := &errorMsg{msg: err.Error()}
			_, _ = disp.conn.Write(errMsg.Bytes())
			return
		}
	}
}

func (d *daemon) removeDispatcher(disp *dispatcher) {
	slog.Info("removing dispatcher", "addr", disp.conn.RemoteAddr(), "dispatcher", *disp)
	disp.kill <- 1
	_ = disp.conn.Close()
	deleteFunc := func(a *dispatcher) bool {
		return a == disp
	}
	d.dispLock.Lock()
	defer d.dispLock.Unlock()
	d.dispatchers = slices.DeleteFunc(d.dispatchers, deleteFunc)
}

func (d *daemon) stopHeartbeat(conn *net.TCPConn) {
	d.hbLock.Lock()
	defer d.hbLock.Unlock()

	slog.Info("stopping heartbeat", "addr", conn.RemoteAddr())
	done, ok := d.heartbeats[conn]
	if ok {
		close(done)
	} else {
		slog.Info("heartbeat already stopped", "addr", conn.RemoteAddr())
	}
}

type camera struct {
	conn  *net.TCPConn
	road  uint16
	mile  uint16
	limit uint16
}

type dispatcher struct {
	conn  *net.TCPConn
	roads []uint16
	comm  chan *ticket
	kill  chan int
}

func newDispatcher(conn *net.TCPConn, roads []uint16) *dispatcher {
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
	plate, _ := parseFixedStr(input, offset) // don't care about the error since it's only used in tests
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

func closeConn(conn *net.TCPConn) {
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
	builder := new(strings.Builder)
	for _, r := range records {
		fmt.Fprintf(builder, "%+v ", *r)
	}

	return builder.String()
}

func sendErrMsg(conn *net.TCPConn, msg string) {
	errMsg := &errorMsg{msg: msg}
	_, _ = conn.Write(errMsg.Bytes())
}
