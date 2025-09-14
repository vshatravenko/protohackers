package main

import (
	"encoding/binary"
	"io"
	"log/slog"
	"net"

	"github.com/vshatravenko/protohackers/internal/server"
)

const (
	reqSize = 6
)

func main() {
	st := newStore()
	srv, err := server.NewTCPServerFromEnv(handler(st))
	if err != nil {
		slog.Error("Could not create TCPServer", "err", err.Error())
	}

	slog.Info("[means_end] Listening for connections", "addr", srv.Addr)
	if err = srv.Start(); err != nil {
		slog.Error("Could not start TCPServer", "err", err.Error())
	}
}

func handler(st *store) func(conn net.Conn) {
	return func(conn net.Conn) {
		defer func() {
			if err := conn.Close(); err != nil {
				slog.Error("Could not close conn", "addr", conn.RemoteAddr())
			}
		}()

		slog.Info("Handling new conn", "addr", conn.RemoteAddr())

		id := st.newSession()
		slog.Info("Started new session", "addr", conn.RemoteAddr(), "id", id)
		defer func() {
			slog.Info("Deleted session", "addr", conn.RemoteAddr(), "id", id)
			st.deleteSession(id)
		}()

		for {
			buf := make([]byte, reqSize)
			n, err := conn.Read(buf)
			if err == io.EOF {
				slog.Info("Finished reading conn", "addr", conn.RemoteAddr())
				return
			} else if err != nil {
				slog.Error("Could not read from conn", "addr", conn.RemoteAddr())
				return
			}

			if n != reqSize {
				slog.Error("Incomplete payload", "addr", conn.RemoteAddr(), "payload", buf[:n], "len", n)
			}

			p, err := parseRawPayload(buf)
			if err != nil {
				slog.Error("Payload parsing failed", "addr", conn.RemoteAddr(), "err", err.Error(), "payload", buf)
			}

			if p.action == 'I' {
				st.insert(id, newRecord(p.segment1, p.segment2))
			} else {
				mean := st.mean(id, p.segment1, p.segment2)
				respBuf := make([]byte, 4)
				binary.BigEndian.PutUint32(respBuf, uint32(mean))

				_, err := conn.Write(respBuf)
				if err != nil {
					slog.Error("Could not write response to conn", "addr", conn.RemoteAddr(), "err", err.Error())
				}
			}
		}
	}
}
