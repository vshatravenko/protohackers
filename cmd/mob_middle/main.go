package main

import (
	"io"
	"log/slog"
	"net"
	"regexp"
	"strings"

	"github.com/vshatravenko/protohackers/internal/server"
)

const (
	initBufSize       = 4096
	minUsernameLength = 1
	maxUsernameLength = 16
	upstreamAddr      = "chat.protohackers.com:16963"
)

func main() {
	srv, err := server.NewTCPServerFromEnv(handler)
	if err != nil {
		slog.Error("Could not create TCPServer", "err", err.Error())
	}

	slog.Info("[mob_middle] Listening for connections", "addr", srv.Addr)
	if err = srv.Start(); err != nil {
		slog.Error("Could not start TCPServer", "err", err.Error())
	}
}

func handler(conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil && err != io.EOF {
			slog.Info("Failed to close connection", "addr", conn.RemoteAddr(), "err", err)
		}
	}()

	upstreamConn, err := net.Dial("tcp4", upstreamAddr)
	if err != nil {
		slog.Warn("Failed to open upstream conn", "addr", upstreamConn.RemoteAddr(), "err", err.Error())
		return
	}

	buf := make([]byte, initBufSize)
	n, err := upstreamConn.Read(buf)
	if err != nil {
		slog.Warn("Failed to read from upstream conn", "addr", upstreamConn.RemoteAddr(), "err", err.Error())
		_ = upstreamConn.Close()
		return
	}

	upstreamWelcomeMsg := buf[:n]

	_, err = conn.Write(upstreamWelcomeMsg)
	if err != nil {
		slog.Info("Failed to write to client", "addr", conn.RemoteAddr(), "err", err.Error(), "msg", string(upstreamWelcomeMsg))
		return
	}

	n, err = conn.Read(buf)
	if err != nil && err != io.EOF {
		slog.Info("Failed to read the username", "addr", conn.RemoteAddr(), "err", err.Error())
		return
	}

	initPayload := buf[:n]
	username := string(initPayload)
	username, _ = strings.CutSuffix(username, "\n")

	if len(username) < minUsernameLength {
		slog.Info("Username is too short", "addr", conn.RemoteAddr(), "username", username)
		_, _ = conn.Write([]byte("error: username is too short"))
		return
	}

	if len(username) > maxUsernameLength {
		slog.Info("Username is too long", "addr", conn.RemoteAddr(), "username", username)
		_, _ = conn.Write([]byte("error: username is too long"))
		return
	}

	validUsername := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	if !validUsername.MatchString(username) {
		slog.Info("Non-alphanumeric username detected", "addr", conn.RemoteAddr(), "username", username)
		_, _ = conn.Write([]byte("error: only alphanumeric usernames are allowed"))
		return
	}

	if _, err = upstreamConn.Write(initPayload); err != nil {
		slog.Warn("Failed to register on upstream", "addr", conn.RemoteAddr(), "username", username, "err", err.Error())
		return
	}

	client := newClient(username, conn, upstreamConn)
	client.start()
	client.waitToStop()
}
