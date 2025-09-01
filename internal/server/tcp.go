package server

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
)

const (
	ipEnvVar    = "PROTO_IP"
	portEnvVar  = "PROTO_PORT"
	defaultPort = 4269
	netEnvVar   = "PROTO_NET"
	defaultNet  = "tcp4"
)

var LocalIP = net.IPv4(0, 0, 0, 0)

type TCPServer struct {
	Addr    *net.TCPAddr
	Network string
	handler func(net.Conn)
}

func configureTCPAddr(ip net.IP, port int) *net.TCPAddr {
	return &net.TCPAddr{
		IP:   ip,
		Port: port,
	}
}

func NewTCPServerFromEnv(handler func(net.Conn)) (*TCPServer, error) {
	var err error
	var ip net.IP
	ipRaw, ok := os.LookupEnv(ipEnvVar)
	if !ok {
		ip = LocalIP
	} else {
		ip = net.ParseIP(ipRaw)
		if ip == nil {
			return nil, fmt.Errorf("%s is not a valid IP address", ipRaw)
		}
	}

	var port int
	portRaw, ok := os.LookupEnv(portEnvVar)
	if !ok {
		port = defaultPort
	} else {
		port, err = strconv.Atoi(portRaw)
		if err != nil {
			return nil, fmt.Errorf("%s is not a valid port number", portRaw)
		}
	}

	network, ok := os.LookupEnv(netEnvVar)
	if !ok {
		network = defaultNet
	}

	return NewTCPServer(ip, port, network, handler), nil
}

func NewTCPServer(ip net.IP, port int, network string, handler func(net.Conn)) *TCPServer {
	addr := configureTCPAddr(ip, port)

	return &TCPServer{
		Addr:    addr,
		Network: network,
		handler: handler,
	}
}

func (ts *TCPServer) Start() error {
	listener, err := net.ListenTCP(ts.Network, ts.Addr)
	if err != nil {
		return err
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			slog.Info("new connection failed", "err", err.Error())
			continue
		}

		slog.Info("Handling conn", "addr", conn.RemoteAddr())
		go ts.handler(conn)
	}
}
