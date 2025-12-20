package main

import (
	"encoding/binary"
	"net"
)

type daemon struct {
	cameras     map[uint16]*camera
	dispatchers []*dispatcher
	records     map[string][]record
	tickets     map[string][]ticket
}

func newDaemon() *daemon {
	return &daemon{
		cameras:     make(map[uint16]*camera),
		dispatchers: make([]*dispatcher, 0),
		records:     make(map[string][]record),
		tickets:     make(map[string][]ticket),
	}
}

func (d *daemon) addCamera(conn *net.Conn, road, mile, limit uint16) {
	d.cameras[road] = &camera{conn: conn, road: road, mile: mile, limit: limit}
}

func (d *daemon) addDispatcher(conn *net.Conn, roads []uint16) {
	d.dispatchers = append(d.dispatchers, &dispatcher{conn: conn, roads: roads})
}

type camera struct {
	conn  *net.Conn
	road  uint16
	mile  uint16
	limit uint16
}

type dispatcher struct {
	conn  *net.Conn
	roads []uint16
}

type record struct {
	plate     string
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

	ts1 := make([]byte, 4)
	binary.BigEndian.PutUint32(ts1, t.timestamp1)
	res = append(res, ts1...)

	mile1 := make([]byte, 2)
	binary.BigEndian.PutUint16(mile1, t.mile1)
	res = append(res, mile1...)

	ts2 := make([]byte, 4)
	binary.BigEndian.PutUint32(ts2, t.timestamp2)
	res = append(res, ts2...)

	mile2 := make([]byte, 2)
	binary.BigEndian.PutUint16(mile2, t.mile2)
	res = append(res, mile2...)

	road := make([]byte, 2)
	binary.BigEndian.PutUint16(road, t.road)
	res = append(res, road...)

	speed := make([]byte, 2)
	binary.BigEndian.PutUint16(speed, t.speed)
	res = append(res, speed...)

	return res
}
