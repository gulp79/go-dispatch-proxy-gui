//go:build linux

package main

import (
	"net"
	"syscall"
	"time"
)

func DialBackend(d *Dispatcher, remoteAddr string) (net.Conn, *Backend, int, error) {
	lb, idx := d.Next()
	localAddr, err := net.ResolveTCPAddr("tcp4", lb.Address)
	if err != nil {
		return nil, lb, idx, err
	}

	dialer := net.Dialer{
		LocalAddr: localAddr,
		Timeout:   10 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				if lb.Interface != "" {
					syscall.BindToDevice(int(fd), lb.Interface)
				}
			})
		},
	}
	c, err := dialer.Dial("tcp4", remoteAddr)
	return c, lb, idx, err
}