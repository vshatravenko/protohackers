package main

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"syscall"
	"time"
)

const (
	defaultNet     = "tcp4"
	defaultPort    = 4269
	defaultAddr    = "0.0.0.0:4269"
	defaultBufSize = 128
	defaultTTL     = 60 * time.Second
)

var defaultIP = net.IPv4(0, 0, 0, 0)

func main() {
	addr := configureTCPAddr(defaultIP, defaultPort)
	slog.Info("Listening for new connections", "addr", addr)
	listener, err := net.ListenTCP(defaultNet, addr)
	if err != nil {
		slog.Info("ListenTCP failed", "err", err.Error())
		os.Exit(1)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			slog.Info("new connection failed", "err", err.Error())
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil {
			slog.Error("Failed to close a conn", "addr", conn.RemoteAddr(), "err", err.Error())
		}
	}()

	slog.Info("Handling new conn", "addr", conn.RemoteAddr())

	if err := conn.SetDeadline(time.Now().Add(defaultTTL)); err != nil {
		slog.Error("Failed to set a deadline for a conn", "addr", conn.RemoteAddr(), "err", err.Error())
		return
	}

	inProgress := true
	for inProgress {
		buf := make([]byte, defaultBufSize)
		count, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				slog.Info("Finished reading from conn", "addr", conn.RemoteAddr())
				inProgress = false
			} else {
				if errors.Is(err, syscall.ECONNRESET) {
					slog.Info("Closed conn", "addr", conn.RemoteAddr(), "reason", "reset by peer")
				} else {
					slog.Warn("Failed to read from conn", "addr", conn.RemoteAddr(), "err", err.Error())
				}

				return
			}
		}

		slog.Info("Read bytes from conn", "addr", conn.RemoteAddr(), "bytes", string(buf), "count", count)

		count, err = conn.Write(buf)
		if err != nil {
			if err != io.EOF {
				slog.Warn("Failed to write to conn", "addr", conn.RemoteAddr(), "err", err.Error())
				return
			}

			slog.Info("Finished writing to conn", "addr", conn.RemoteAddr())
		}
		slog.Info("Wrote bytes to conn", "addr", conn.RemoteAddr(), "bytes", string(buf), "count", count)

	}
}

func configureTCPAddr(ip net.IP, port int) *net.TCPAddr {
	return &net.TCPAddr{
		IP:   ip,
		Port: port,
	}
}
