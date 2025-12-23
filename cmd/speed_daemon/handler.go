package main

import (
	"log/slog"
	"net"
)

func (d *daemon) handleConn(conn net.Conn, payload []byte) {
	msgType := uint8(payload[0])
	payload = payload[1:]

	switch msgType {
	case msgTypes["error"]:
		slog.Debug("parsed error msg", "addr", conn.RemoteAddr(), "msg", parseErrorMsg(payload))
	// case msgTypes["plate"]:
	//	msg := parsePlateMsg(payload)
	//	slog.Debug("parsed plate msg", "addr", conn.RemoteAddr(), "msg", msg)
	case msgTypes["ticket"]: // this will never be sent initially
		slog.Debug("parsing ticket msg")
	case msgTypes["want_heartbeat"]: // ditto
		msg := parseWantHeartbeatMsg(payload)
		slog.Debug("parsed want_heartbeat msg", "addr", conn.RemoteAddr(), "msg", msg)
	case msgTypes["heartbeat"]: // this is only sent by the server
		slog.Debug("parsing heartbeat msg")
	case msgTypes["camera"]:
		slog.Debug("parsed camera msg", "addr", conn.RemoteAddr())
		d.handleCamera(conn, payload[1:])
	case msgTypes["dispatcher"]:
		msg := parseDispatcherMsg(payload)
		slog.Debug("parsed dispatcher msg", "addr", conn.RemoteAddr(), "msg", msg)
		d.addDispatcher(conn, msg.roads)
	default:
		slog.Info("received unknown msg", "payload", payload)
	}
}
