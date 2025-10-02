package main

import (
	"bufio"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"

	"github.com/vshatravenko/protohackers/internal/server"
)

const (
	reqSize = 9
	bufSize = 100000
)

func main() {
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/profile", pprof.Profile)
		err := http.ListenAndServe(":7777", mux)
		if err != nil {
			slog.Error("failed to serve profiler HTTP server", "err", err)
		}
	}()

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
	buf := make([]byte, 9)
	reader := bufio.NewReaderSize(conn, bufSize)
	for {
		_, err := io.ReadFull(reader, buf)
		if err == io.EOF {
			slog.Info("Finished reading conn", "addr", conn.RemoteAddr())
			return
		} else if err != nil {
			slog.Error("Could not read from conn", "addr", conn.RemoteAddr(), "err", err.Error())
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
			slog.Info("Started calculating mean", "startDate", p.segment1, "endDate", p.segment2)
			mean := st.mean(p.segment1, p.segment2)
			slog.Info("Finished calculating mean", "startDate", p.segment1, "endDate", p.segment2)

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
