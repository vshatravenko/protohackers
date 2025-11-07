package server

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
)

type UDPServer struct {
	Addr    *net.UDPAddr
	Network string
	handler func(*net.UDPConn)
}

func configureUDPAddr(ip net.IP, port int) *net.UDPAddr {
	return &net.UDPAddr{
		IP:   ip,
		Port: port,
	}
}

func NewUDPServerFromEnv(handler func(*net.UDPConn)) (*UDPServer, error) {
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
		port = DefaultPort
	} else {
		port, err = strconv.Atoi(portRaw)
		if err != nil {
			return nil, fmt.Errorf("%s is not a valid port number", portRaw)
		}
	}

	network, ok := os.LookupEnv(netEnvVar)
	if !ok {
		network = defaultTCPNet
	}

	return NewUDPServer(ip, port, network, handler), nil
}

func NewUDPServer(ip net.IP, port int, network string, handler func(*net.UDPConn)) *UDPServer {
	addr := configureUDPAddr(ip, port)

	return &UDPServer{
		Addr:    addr,
		Network: network,
		handler: handler,
	}
}

func (us *UDPServer) Start() error {
	for {
		conn, err := net.ListenUDP(us.Network, us.Addr)
		if err != nil {
			return err
		}

		slog.Info("Handling conn", "addr", conn.RemoteAddr())
		go us.handler(conn)
	}
}
