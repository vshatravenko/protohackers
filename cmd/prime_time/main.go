package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultNet     = "tcp4"
	defaultPort    = 4269
	defaultAddr    = "0.0.0.0:4269"
	defaultBufSize = 100000
	defaultTTL     = 60 * time.Second
)

var defaultIP = net.IPv4(0, 0, 0, 0)

type unquotedNumber string

func (un unquotedNumber) Int64() (int64, error) {
	return strconv.ParseInt(string(un), 10, 64)
}

func (un unquotedNumber) Float64() (float64, error) {
	return strconv.ParseFloat(string(un), 64)
}

func (un unquotedNumber) IsFloat() bool {
	return strings.Contains(string(un), ".")
}

func (un unquotedNumber) IsQuoted() bool {
	return strings.Contains(string(un), ".")
}

func (un *unquotedNumber) UnmarshalJSON(bytes []byte) error {
	slog.Info("un unmarshalling", "bytes", string(bytes))
	if bytes[0] == '"' {
		return errors.New("failed to parse a number: quote detected")
	}

	strVal := string(bytes)

	if tmp, err := strconv.ParseInt(strVal, 10, 64); err != nil {
		slog.Info("un unmarshalling - not a valid int", "bytes", string(bytes))
		if tmp, err := strconv.ParseFloat(strVal, 64); err != nil {
			return errors.New("not a valid int or float")
		} else {
			slog.Info("un unmarshalling - valid float", "bytes", string(bytes), "float", tmp)
		}
	} else {
		slog.Info("un unmarshalling - valid int", "bytes", string(bytes), "int", tmp)
	}

	*un = unquotedNumber(strVal)

	intVal, err := un.Int64()
	if err != nil {
		slog.Info("un parsed", "bytes", bytes, "int", intVal)
	}

	floatVal, err := un.Float64()
	if err != nil {
		slog.Info("un parsed", "bytes", bytes, "float", floatVal)
	}

	return nil
}

type request struct {
	Method string         `json:"method"`
	Number unquotedNumber `json:"number"`
}

// TODO: handle multiline JSON input
func (r *request) isPrime() bool {
	if r.Number.IsFloat() {
		return false
	}

	num, err := r.Number.Int64()
	if err != nil {
		return false
	}

	if num < 2 {
		return false
	}

	root := int64(math.Sqrt(float64(num)))
	for i := int64(2); i <= root; i++ {
		if num%i == 0 {
			return false
		}
	}

	return true
}

type response struct {
	Method string `json:"method"`
	Prime  bool   `json:"prime"`
}

func main() {
	addr := configureTCPAddr(defaultIP, defaultPort)
	slog.Info("[prime_time] Listening for new connections", "addr", addr)
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

func handleBufferedConn(conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil {
			slog.Warn("Conn close failed", "addr", conn.RemoteAddr(), "err", err.Error())
		}
	}()

	reader := bufio.NewReaderSize(conn, defaultBufSize)

	for {
		req, err := parseLine(reader)
		if err != nil {
			slog.Warn("Could not parse request", "addr", conn.RemoteAddr(), "err", err.Error())

			resp := &response{Method: "err"}

			if err = writeResponse(conn, resp); err != nil {
				slog.Warn("Could not write response", "addr", conn.RemoteAddr(), "err", err.Error())
			}

			return
		}

		resp := &response{
			Method: "isPrime",
			Prime:  req.isPrime(),
		}

		if err = writeResponse(conn, resp); err != nil {
			slog.Warn("Could not write response", "addr", conn.RemoteAddr(), "err", err.Error())
		}
	}

}

func parseLine(reader *bufio.Reader) (*request, error) {
	req := &request{}

	line, isPrefix, err := reader.ReadLine()
	for isPrefix {
		remainder := []byte{}
		remainder, isPrefix, err = reader.ReadLine()
		line = append(line, remainder...)
	}

	if err != nil {
		return nil, err
	}

	slog.Info("Read line", "line", string(line))

	err = json.Unmarshal(line, req)
	if err != nil {
		return nil, err
	}

	if req.Method != "isPrime" {
		return nil, fmt.Errorf("%s is not a valid method, expecting 'isPrime'", req.Method)
	}

	if req.Number == "" {
		return nil, errors.New("'number' is missing")
	}

	return req, nil
}

func writeResponse(conn net.Conn, resp *response) error {
	respBytes, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	_, err = conn.Write(append(respBytes, '\n'))

	return err
}

func configureTCPAddr(ip net.IP, port int) *net.TCPAddr {
	return &net.TCPAddr{
		IP:   ip,
		Port: port,
	}
}
