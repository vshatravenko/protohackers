package main

import (
	"log/slog"
	"net"
	"os"
	"strings"

	"github.com/vshatravenko/protohackers/internal/server"
)

const (
	bufSize = 1000
)

var (
	versionMsg = []byte("version=Pretty pretty pretty nice database")
)

func main() {
	db := newDatabase()
	srv, err := server.NewUDPServerFromEnv(handler(db))
	if err != nil {
		slog.Error("server init failed", "err", err.Error())
		os.Exit(1)
	}

	slog.Info("[unusual_database] listening for UDP connections", "addr", srv.Addr)
	if err := srv.Start(); err != nil {
		slog.Error("server error", "err", err.Error())
		os.Exit(1)
	}
}

func handler(db *database) func(net.PacketConn) {
	return func(conn net.PacketConn) {
		buf := make([]byte, bufSize)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				slog.Warn("read from UDP conn failed", "addr", addr, "err", err.Error())
			}

			if n == 0 {
				slog.Warn("empty payload read", "addr", addr)
			}

			slog.Info("handling non-zero read", "addr", addr, "buf", string(buf[:n]))
			payload := string(buf[:n])

			if strings.Contains(payload, "=") {
				components := strings.SplitN(payload, "=", 2)
				key, value := components[0], components[1]

				if key == "version" {
					continue
				}

				db.insert(key, value)
			} else {
				var response []byte

				if payload == "version" {
					response = versionMsg
				} else {
					value := db.retrieve(payload)
					response = []byte(payload + "=" + value)
				}

				if _, err := conn.WriteTo(response, addr); err != nil {
					slog.Warn("failed writing the response", "addr", addr, "err", err.Error())
				}

				slog.Info("wrote response", "addr", addr, "response", response)
			}
		}
	}

}
