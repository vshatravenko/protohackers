package main

import (
	"log/slog"
	"net"

	"github.com/vshatravenko/protohackers/internal/logger"
	"github.com/vshatravenko/protohackers/internal/server"
)

const (
	bufSize = 512
)

func main() {
	logger.ConfigureDefaultLoggerFromEnv()

	d := newDaemon()
	go d.processPendingTickets()

	srv, err := server.NewTCPServerFromEnv(handler(d))
	if err != nil {
		slog.Error("Could not create TCPServer", "err", err.Error())

	}

	slog.Info("[speed_daemon] Listening for connections", "addr", srv.Addr)
	if err = srv.Start(); err != nil {
		slog.Error("Could not start TCPServer", "err", err.Error())
	}
}

/*
TODO:
1. The camera & dispatcher registration flow
2. The speed limit violation calculation logic
3. Keep track of speed records by car
4. Track non-contiguous violations
5. Avoid issuing more than one ticket per car per day
6. Reliably deliver ticket messages to dispatchers
*/
func handler(d *daemon) func(net.Conn) {
	return func(conn net.Conn) {
		defer func() {
			_ = conn.Close()
		}()

		b := make([]byte, bufSize)

		n, err := conn.Read(b)
		if err != nil {
			slog.Warn("initial conn read failed", "addr", conn.RemoteAddr(), "err", err.Error())
			return
		}

		payload := b[:n]
		d.handleConn(conn, payload)
	}
}
