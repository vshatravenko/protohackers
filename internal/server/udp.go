package server

import (
	"net"
	"os"
)

type UDPServer struct {
	Addr    string
	Network string
	handler func(net.PacketConn)
}

func configureUDPAddr(ip net.IP, port int) *net.UDPAddr {
	return &net.UDPAddr{
		IP:   ip,
		Port: port,
	}
}

func NewUDPServerFromEnv(handler func(net.PacketConn)) (*UDPServer, error) {
	var addr string
	addr, ok := os.LookupEnv(udpAddrEnvVar)
	if !ok {
		addr = "0.0.0.0:4269"
	}

	network, ok := os.LookupEnv(udpNetEnvVar)
	if !ok {
		network = defaultUDPNet
	}

	return NewUDPServer(addr, network, handler), nil
}

func NewUDPServer(addr string, network string, handler func(net.PacketConn)) *UDPServer {

	return &UDPServer{
		Addr:    addr,
		Network: network,
		handler: handler,
	}
}

func (us *UDPServer) Start() error {
	conn, err := net.ListenPacket(us.Network, us.Addr)
	if err != nil {
		return err
	}

	us.handler(conn)

	return nil
}
