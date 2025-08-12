package main

import (
	"bufio"
	"io"
	"log/slog"
	"net"
	"os"
	"time"
)

const (
	defaultNet     = "tcp4"
	defaultPort    = 4269
	defaultAddr    = "0.0.0.0:4269"
	defaultBufSize = 4096
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

		slog.Info("Handling conn", "addr", conn.RemoteAddr())
		go handleBufferedConn(conn)
	}
}

// I'd like to read the connection in a buffered way
// and only start writing back after the whole thing is read
func handleBufferedConn(conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil {
			slog.Warn("Conn close failed", "addr", conn.RemoteAddr(), "err", err.Error())
		}
	}()

	reader := bufio.NewReader(conn)
	buf := make([]byte, defaultBufSize)

	for {
		slog.Info("Reading from conn", "addr", conn.RemoteAddr())
		n, err := reader.Read(buf)
		if err == io.EOF {
			return
		} else if err != nil {
			slog.Error("Could not read from conn", "addr", conn.RemoteAddr(), "err", err.Error())
		}

		slog.Info("Writing to conn", "addr", conn.RemoteAddr())
		if _, err = conn.Write(buf[:n]); err != nil {
			slog.Error("Could not write to conn", "addr", conn.RemoteAddr(), "err", err.Error())
		}
	}

}

func configureTCPAddr(ip net.IP, port int) *net.TCPAddr {
	return &net.TCPAddr{
		IP:   ip,
		Port: port,
	}
}
