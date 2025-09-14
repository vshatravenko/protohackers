package main

import (
	"encoding/binary"
	"io"
	"log/slog"
	"net"

	"github.com/vshatravenko/protohackers/internal/server"
)

const (
	reqSize = 9
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

	st := newStore()
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
			return
		}

		p, err := parseRawPayload(buf)
		if err != nil {
			slog.Error("Payload parsing failed", "addr", conn.RemoteAddr(), "err", err.Error(), "payload", buf)
			return
		}

		switch p.action {
		case actionInsert:
			st.insert(newRecord(p.segment1, p.segment2))
		case actionQuery:
			mean := st.mean(p.segment1, p.segment2)
			respBuf := make([]byte, 4)
			binary.BigEndian.PutUint32(respBuf, uint32(mean))

			_, err := conn.Write(respBuf)
			if err != nil {
				slog.Error("Could not write response to conn", "addr", conn.RemoteAddr(), "err", err.Error())
				return
			}
		default:
			slog.Warn("Unknown action, closing conn", "addr", conn.RemoteAddr(), "action", string(p.action))
			return
		}
	}
}
