package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"net"
)

const (
	SocksVersion5 = 0x05
	CmdConnect    = 0x01
	AddrTypeIPv4  = 0x01
	AddrTypeDom   = 0x03
	AddrTypeIPv6  = 0x04
)

func (s *ProxyServer) handleSocks(conn net.Conn) {
	defer conn.Close()

	// 1. Handshake
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return
	}
	if buf[0] != SocksVersion5 {
		return
	}
	methods := make([]byte, int(buf[1]))
	io.ReadFull(conn, methods)
	conn.Write([]byte{SocksVersion5, 0x00}) // No Auth

	// 2. Request
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return
	}
	if header[1] != CmdConnect {
		conn.Write([]byte{SocksVersion5, 0x07, 0, 1, 0, 0, 0, 0, 0, 0}) // Cmd not supported
		return
	}

	var dest string
	portBuf := make([]byte, 2)
	switch header[3] {
	case AddrTypeIPv4:
		ip := make([]byte, 4)
		io.ReadFull(conn, ip)
		io.ReadFull(conn, portBuf)
		dest = fmt.Sprintf("%s:%d", net.IP(ip).String(), binary.BigEndian.Uint16(portBuf))
	case AddrTypeDom:
		l := make([]byte, 1)
		io.ReadFull(conn, l)
		dom := make([]byte, int(l[0]))
		io.ReadFull(conn, dom)
		io.ReadFull(conn, portBuf)
		dest = fmt.Sprintf("%s:%d", string(dom), binary.BigEndian.Uint16(portBuf))
	case AddrTypeIPv6:
		ip := make([]byte, 16)
		io.ReadFull(conn, ip)
		io.ReadFull(conn, portBuf)
		dest = fmt.Sprintf("[%s]:%d", net.IP(ip).String(), binary.BigEndian.Uint16(portBuf))
	}

	// 3. Dial Backend
	remote, lb, idx, err := DialBackend(s.dispatcher, dest)
	if err != nil {
		s.log(fmt.Sprintf("[WARN] Connect fail %s: %v", dest, err))
		conn.Write([]byte{SocksVersion5, 0x04, 0, 1, 0, 0, 0, 0, 0, 0}) // Host unreachable
		return
	}
	
	s.log(fmt.Sprintf("[DEBUG] SOCKS %s -> %s (via %s LB:%d)", conn.RemoteAddr(), dest, lb.Address, idx))
	conn.Write([]byte{SocksVersion5, 0x00, 0, 1, 0, 0, 0, 0, 0, 0}) // Success
	pipe(conn, remote)
}

func (s *ProxyServer) handleTunnel(conn net.Conn) {
	defer conn.Close()
	failedBits := big.NewInt(0)
	
	for {
		lb, idx := s.dispatcher.GetNextFailed(failedBits)
		if lb == nil {
			s.log("[WARN] Tunnel: all backends failed")
			return
		}

		remote, _, _, err := DialBackend(s.dispatcher, lb.Address) // lb.Address è target in tunnel mode
		if err == nil {
			s.log(fmt.Sprintf("[DEBUG] Tunnel -> %s (LB:%d)", lb.Address, idx))
			pipe(conn, remote)
			return
		}
		
		s.log(fmt.Sprintf("[WARN] Tunnel fail %s (LB:%d): %v", lb.Address, idx, err))
		failedBits.SetBit(failedBits, idx, 1)
	}
}