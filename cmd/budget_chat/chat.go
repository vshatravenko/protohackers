package main

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
)

// TODO: user buffered channels

const (
	containsMsgPrefix = "* The room contains: "
)

type chat struct {
	lobby map[string]*user
	mtx   *sync.RWMutex
}

type user struct {
	username string
	conn     net.Conn
	messages chan string
}

func newUser(username string, conn net.Conn) *user {
	return &user{
		username: username,
		conn:     conn,
		messages: make(chan string, 10),
	}
}

func (c *chat) writeTo(username string, wg *sync.WaitGroup) {
	defer wg.Done()

	c.mtx.RLock()
	u := c.lobby[username]
	c.mtx.RUnlock()

	for msg := range u.messages {
		if _, err := u.conn.Write([]byte(msg)); err != nil {
			slog.Info("Failed writing to user conn", "username", u.username, "msg", msg, "err", err.Error(), "addr", u.conn.RemoteAddr())
			continue
		}
	}
}

func (c *chat) readFrom(username string, wg *sync.WaitGroup) {
	defer wg.Done()

	c.mtx.RLock()
	conn := c.lobby[username].conn
	c.mtx.RUnlock()

	for {
		reader := bufio.NewReaderSize(conn, bufSize)
		line, _, err := reader.ReadLine()
		if err != nil {
			if err != io.EOF {
				slog.Warn("Failed to read client line", "addr", conn.RemoteAddr(), "err", err.Error())
			}

			if err := conn.Close(); err != nil {
				slog.Info("Failed to close connection", "addr", conn.RemoteAddr(), "err", err.Error())
			}
			c.leave(username)
			return
		}

		msg := c.formatMsg(username, string(line))
		c.broadcast(username, msg)
	}
}

func newChat() *chat {
	return &chat{
		lobby: map[string]*user{},
		mtx:   &sync.RWMutex{}}
}

func (c *chat) hasUser(username string) bool {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	_, present := c.lobby[username]
	return present
}

func (c *chat) broadcast(from, message string) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	for name, u := range c.lobby {
		if name != from {
			u.messages <- message
		}
	}
}

func (c *chat) sendTo(username, message string) {
	c.mtx.RLock()
	slog.Info("Sent direct message", "username", username, "msg", message)
	c.lobby[username].messages <- message
	c.mtx.RUnlock()
}

func (c *chat) join(username string, conn net.Conn) bool {
	slog.Info("joining the chat", "username", username)

	if c.hasUser(username) {
		slog.Info("Duplicate username", "addr", conn.RemoteAddr(), "username", username)
		return false
	}

	c.mtx.Lock()
	c.lobby[username] = newUser(username, conn)
	c.mtx.Unlock()

	c.sendTo(username, c.formatContainsMsg(username))
	c.broadcast(username, c.formatJoinedMsg(username))

	return true
}

func (c *chat) leave(username string) {
	if !c.hasUser(username) {
		return
	}

	c.broadcast(username, c.formatLeftMsg(username))
	c.mtx.Lock()
	delete(c.lobby, username)
	c.mtx.Unlock()
}

func (c *chat) formatMsg(from, message string) string {
	return fmt.Sprintf("[%s] %s\n", from, message)
}

func (c *chat) formatContainsMsg(from string) string {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	res := containsMsgPrefix

	for username := range c.lobby {
		if username != from {
			res += username + ", "
		}
	}

	if len(res) > len(containsMsgPrefix) {
		res = res[:len(res)-2]
	}

	return res + "\n"
}

func (c *chat) formatJoinedMsg(from string) string {
	return fmt.Sprintf("* %s has entered the room\n", from)
}

func (c *chat) formatLeftMsg(from string) string {
	return fmt.Sprintf("* %s has left the room\n", from)
}
