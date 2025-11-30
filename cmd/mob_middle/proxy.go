package main

import (
	"io"
	"log/slog"
	"net"
	"strings"
	"time"
)

const (
	clientBufSize        = 4096
	upstreamBufSize      = 4096
	waitToStopInterval   = 1
	replacementBogusAddr = "7YWHMfk9JZe0LM0g1ZauHuiSxhI"
	bogusAddrStartByte   = '7'
	bogusAddrMinLen      = 26
	bogusAddrMaxLen      = 35
)

type client struct {
	selfConn     net.Conn
	upstreamConn net.Conn
	username     string
	stop         chan int
}

func newClient(username string, selfConn, upstreamConn net.Conn) *client {
	return &client{
		username:     username,
		selfConn:     selfConn,
		upstreamConn: upstreamConn,
		stop:         make(chan int),
	}
}

func (c *client) start() {
	go c.read()
	go c.syncUpstream()
}

func (c *client) read() {
	defer func() { c.stop <- 1 }()

	buf := make([]byte, clientBufSize)
	for {
		isCompleteMessage := false

		payload := []byte{}
		for !isCompleteMessage {
			n, err := c.selfConn.Read(buf)
			if err != nil {
				if err != io.EOF {
					slog.Warn("client read failed", "addr", c.selfConn.RemoteAddr(), "err", err.Error())
				}

				return
			}

			if n == 0 {
				slog.Warn("received empty client payload", "username", c.username, "addr", c.upstreamConn.RemoteAddr())
				return
			}

			payload = append(payload, buf[:n]...)
			isCompleteMessage = strings.Contains(string(payload), "\n")
		}

		payload = replaceBogusAddr(payload)
		if _, err := c.upstreamConn.Write(payload); err != nil {
			slog.Warn("upstream write failed", "addr", c.upstreamConn.RemoteAddr(), "err", err.Error())
			return
		}

		slog.Info("client payload", "username", c.username, "payload", string(payload))
	}
}

func (c *client) syncUpstream() {
	defer func() { c.stop <- 1 }()
	buf := make([]byte, upstreamBufSize)

	for {
		n, err := c.upstreamConn.Read(buf)
		if err != nil && err != io.EOF {
			slog.Warn("upstream read failed", "username", c.username, "addr", c.upstreamConn.RemoteAddr(), "err", err.Error())
			break
		}

		if n == 0 {
			slog.Warn("received empty upstream payload", "username", c.username, "addr", c.upstreamConn.RemoteAddr(), "err", err.Error())
			break
		}

		payload := replaceBogusAddr(buf[:n])
		_, err = c.selfConn.Write(payload)
		if err != nil {
			slog.Warn("client write failed", "addr", c.selfConn.RemoteAddr(), "err", err.Error())
			break
		}

		slog.Info("upstream payload", "username", c.username, "payload", string(payload))
	}
}

func (c *client) waitToStop() {
	for {
		select {
		case <-c.stop:
			slog.Info("Encountered stop signal, closing connections", "username", c.username, "addr", c.selfConn.RemoteAddr())
			_ = c.selfConn.Close()
			_ = c.upstreamConn.Close()
			return
		case <-time.After(time.Second * waitToStopInterval):
		}
	}
}

func replaceBogusAddr(input []byte) []byte {
	res := input

	for i := 0; i < len(res); i++ {
		if res[i] == bogusAddrStartByte && (i == 0 || res[i-1] == ' ') {
			start, end := i, i+1
			for end < len(res) && isAlphaNum(res[end]) {
				end++
			}

			isValidLength := (end-start) >= bogusAddrMinLen && (end-start) <= bogusAddrMaxLen

			if isValidLength && (end == len(res) || res[end] == ' ' || res[end] == '\n') {
				res, i = replaceWithSequence(res, []byte(replacementBogusAddr), start, end)
			} else {
				i = end
			}
		}
	}

	return res
}

func replaceWithSequence(input, sequence []byte, start, end int) ([]byte, int) {
	preSeq := make([]byte, len(input[:start]))
	postSeq := make([]byte, len(input[end:]))
	copy(preSeq, input[:start])
	copy(postSeq, input[end:])

	res := append(preSeq, sequence...)
	offset := len(res)
	res = append(res, postSeq...)

	return res, offset
}

func isAlphaNum(a byte) bool {
	isNum := a >= '0' && a <= '9'
	isLowerAlpha := a >= 'a' && a <= 'z'
	isUpperAlpha := a >= 'A' && a <= 'Z'

	return isNum || isLowerAlpha || isUpperAlpha
}
