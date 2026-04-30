package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"net"
	"strconv"
)

const (
	SocksVersion4 = 0x04
	SocksVersion5 = 0x05
	CmdConnect    = 0x01
	AddrTypeIPv4  = 0x01
	AddrTypeDom   = 0x03
	AddrTypeIPv6  = 0x04

	socks5NoAuth              = 0x00
	socks5NoAcceptableMethods = 0xff
	socks5Success             = 0x00
	socks5GeneralFailure      = 0x01
	socks5HostUnreachable     = 0x04
	socks5CommandUnsupported  = 0x07
	socks5AddrUnsupported     = 0x08

	socks4RequestGranted  = 0x5a
	socks4RequestRejected = 0x5b
)

func (s *ProxyServer) handleProxy(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	firstByte, err := reader.Peek(1)
	if err != nil {
		return
	}

	switch firstByte[0] {
	case SocksVersion5:
		s.handleSocks5(conn, reader)
	case SocksVersion4:
		s.handleSocks4(conn, reader)
	default:
		s.handleHTTPProxy(conn, reader)
	}
}

func (s *ProxyServer) handleSocks5(conn net.Conn, reader *bufio.Reader) {
	buf := make([]byte, 2)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return
	}
	if buf[0] != SocksVersion5 {
		return
	}

	methods := make([]byte, int(buf[1]))
	if _, err := io.ReadFull(reader, methods); err != nil {
		return
	}

	hasNoAuth := false
	for _, method := range methods {
		if method == socks5NoAuth {
			hasNoAuth = true
			break
		}
	}
	if !hasNoAuth {
		conn.Write([]byte{SocksVersion5, socks5NoAcceptableMethods})
		return
	}
	conn.Write([]byte{SocksVersion5, socks5NoAuth})

	header := make([]byte, 4)
	if _, err := io.ReadFull(reader, header); err != nil {
		return
	}
	if header[0] != SocksVersion5 {
		writeSocks5Reply(conn, socks5GeneralFailure)
		return
	}
	if header[1] != CmdConnect {
		writeSocks5Reply(conn, socks5CommandUnsupported)
		return
	}

	var dest string
	portBuf := make([]byte, 2)
	switch header[3] {
	case AddrTypeIPv4:
		ip := make([]byte, 4)
		if _, err := io.ReadFull(reader, ip); err != nil {
			return
		}
		if _, err := io.ReadFull(reader, portBuf); err != nil {
			return
		}
		dest = fmt.Sprintf("%s:%d", net.IP(ip).String(), binary.BigEndian.Uint16(portBuf))
	case AddrTypeDom:
		l := make([]byte, 1)
		if _, err := io.ReadFull(reader, l); err != nil {
			return
		}
		dom := make([]byte, int(l[0]))
		if _, err := io.ReadFull(reader, dom); err != nil {
			return
		}
		if _, err := io.ReadFull(reader, portBuf); err != nil {
			return
		}
		dest = net.JoinHostPort(string(dom), strconv.Itoa(int(binary.BigEndian.Uint16(portBuf))))
	case AddrTypeIPv6:
		ip := make([]byte, 16)
		if _, err := io.ReadFull(reader, ip); err != nil {
			return
		}
		if _, err := io.ReadFull(reader, portBuf); err != nil {
			return
		}
		dest = net.JoinHostPort(net.IP(ip).String(), strconv.Itoa(int(binary.BigEndian.Uint16(portBuf))))
	default:
		writeSocks5Reply(conn, socks5AddrUnsupported)
		return
	}

	remote, lb, idx, err := DialBackend(s.dispatcher, dest)
	if err != nil {
		s.log(fmt.Sprintf("[WARN] Connect fail %s: %v", dest, err))
		writeSocks5Reply(conn, socks5HostUnreachable)
		return
	}

	s.log(fmt.Sprintf("[DEBUG] SOCKS5 %s -> %s (via %s LB:%d)", conn.RemoteAddr(), dest, lb.Address, idx))
	writeSocks5Reply(conn, socks5Success)
	if err := flushBufferedToRemote(reader, remote); err != nil {
		remote.Close()
		return
	}
	pipe(conn, remote)
}

func (s *ProxyServer) handleSocks4(conn net.Conn, reader *bufio.Reader) {
	header := make([]byte, 8)
	if _, err := io.ReadFull(reader, header); err != nil {
		return
	}
	if header[0] != SocksVersion4 {
		writeSocks4Reply(conn, socks4RequestRejected, header[2:4], header[4:8])
		return
	}
	if header[1] != CmdConnect {
		writeSocks4Reply(conn, socks4RequestRejected, header[2:4], header[4:8])
		return
	}

	port := binary.BigEndian.Uint16(header[2:4])
	ip := net.IP(header[4:8])
	if _, err := readNullTerminated(reader, 4096); err != nil {
		writeSocks4Reply(conn, socks4RequestRejected, header[2:4], header[4:8])
		return
	}

	host := ip.String()
	protocol := "SOCKS4"
	if header[4] == 0 && header[5] == 0 && header[6] == 0 && header[7] != 0 {
		domain, err := readNullTerminated(reader, 4096)
		if err != nil || domain == "" {
			writeSocks4Reply(conn, socks4RequestRejected, header[2:4], header[4:8])
			return
		}
		host = domain
		protocol = "SOCKS4a"
	}

	dest := net.JoinHostPort(host, strconv.Itoa(int(port)))
	remote, lb, idx, err := DialBackend(s.dispatcher, dest)
	if err != nil {
		s.log(fmt.Sprintf("[WARN] Connect fail %s: %v", dest, err))
		writeSocks4Reply(conn, socks4RequestRejected, header[2:4], header[4:8])
		return
	}

	s.log(fmt.Sprintf("[DEBUG] %s %s -> %s (via %s LB:%d)", protocol, conn.RemoteAddr(), dest, lb.Address, idx))
	writeSocks4Reply(conn, socks4RequestGranted, header[2:4], header[4:8])
	if err := flushBufferedToRemote(reader, remote); err != nil {
		remote.Close()
		return
	}
	pipe(conn, remote)
}

func writeSocks5Reply(conn net.Conn, status byte) {
	conn.Write([]byte{SocksVersion5, status, 0, AddrTypeIPv4, 0, 0, 0, 0, 0, 0})
}

func writeSocks4Reply(conn net.Conn, status byte, port []byte, ip []byte) {
	reply := []byte{0x00, status, 0, 0, 0, 0, 0, 0}
	if len(port) >= 2 {
		copy(reply[2:4], port[:2])
	}
	if len(ip) >= 4 {
		copy(reply[4:8], ip[:4])
	}
	conn.Write(reply)
}

func readNullTerminated(reader *bufio.Reader, maxLen int) (string, error) {
	buf := make([]byte, 0, 32)
	for len(buf) < maxLen {
		b, err := reader.ReadByte()
		if err != nil {
			return "", err
		}
		if b == 0x00 {
			return string(buf), nil
		}
		buf = append(buf, b)
	}
	return "", fmt.Errorf("SOCKS4 field too long")
}

func flushBufferedToRemote(reader *bufio.Reader, remote net.Conn) error {
	for reader.Buffered() > 0 {
		pending := make([]byte, reader.Buffered())
		if _, err := io.ReadFull(reader, pending); err != nil {
			return err
		}
		if _, err := remote.Write(pending); err != nil {
			return err
		}
	}
	return nil
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
