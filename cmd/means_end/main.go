package main

import (
	"github.com/vshatravenko/protohackers/internal/server"
	"log/slog"
	"net"
)

func main() {
	srv, err := server.NewTCPServerFromEnv(handler)
	if err != nil {
		slog.Error("Could not create TCPServer", "err", err.Error())
	}

	slog.Info("[means_end] Listening for connections", "addr", srv.Addr)
	if err = srv.Start(); err != nil {
		slog.Error("Could not start TCPServer", "err", err.Error())
	}
}

func handler(conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil {
			slog.Error("Could not close conn", "addr", conn.RemoteAddr())
		}
	}()

	slog.Info("Handling new conn", "addr", conn.RemoteAddr())
}
