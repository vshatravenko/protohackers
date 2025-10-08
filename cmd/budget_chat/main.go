package main

import (
	"io"
	"log/slog"
	"net"
	"regexp"
	"strings"
	"sync"

	"github.com/vshatravenko/protohackers/internal/server"
)

const (
	bufSize           = 1024
	welcomeMsg        = "Welcome to budget_chat! What shall I call you?\n"
	minUsernameLength = 1
	maxUsernameLength = 16
)

var c *chat

func main() {
	srv, err := server.NewTCPServerFromEnv(handler)
	if err != nil {
		slog.Error("Could not create TCPServer", "err", err.Error())
	}

	c = newChat()

	slog.Info("[budget_chat] Listening for connections", "addr", srv.Addr)
	if err = srv.Start(); err != nil {
		slog.Error("Could not start TCPServer", "err", err.Error())
	}
}

/*
	TODO:
	1. Username handling(16 char min, unique) - keep a map of all usernames with the necessary info
	2. Messages broadcast - go through each connection that's not the current user's and send the message
	3. Chat message - broadcast to everyone who's not the current user
	4. Joined message - broadcast to everyone who's joined and not the current user,
	   construct the existing users string
	5. Left message - broadcast to everyone who's joined
*/

// FIXME: empty \n messages for some reason
func handler(conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil && err != io.EOF {
			slog.Info("Failed to close connection", "addr", conn.RemoteAddr(), "err", err)
		}
	}()

	_, err := conn.Write([]byte(welcomeMsg))
	if err != nil {
		slog.Info("Failed to write to connection", "addr", conn.RemoteAddr(), "err", err.Error(), "msg", welcomeMsg)
		return
	}

	buf := make([]byte, bufSize)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		slog.Info("Failed to read the username", "addr", conn.RemoteAddr(), "err", err.Error())
		return
	}

	username := string(buf[:n])
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

	ok := c.join(username, conn)
	if !ok {
		slog.Info("Username is already taken", "addr", conn.RemoteAddr(), "username", username)
		return
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)
	go c.writeTo(username, wg)
	go c.readFrom(username, wg)
	wg.Wait()
}
