package main

import (
	"log/slog"
	"net"

	"github.com/vshatravenko/protohackers/internal/logger"
	"github.com/vshatravenko/protohackers/internal/server"
)

func main() {
	logger.ConfigureDefaultLoggerFromEnv()

	srv, err := server.NewTCPServerFromEnv(handler)
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
func handler(conn net.Conn) {
}
