//go:build !linux

package main

import (
	"net"
	"time"
)

func DialBackend(d *Dispatcher, remoteAddr string) (net.Conn, *Backend, int, error) {
	lb, idx := d.Next()
	localAddr, err := net.ResolveTCPAddr("tcp4", lb.Address)
	if err != nil {
		return nil, lb, idx, err
	}
	// Windows/Mac non supportano BindToDevice facilmente, ci si affida al binding IP
	dialer := net.Dialer{
		LocalAddr: localAddr,
		Timeout:   10 * time.Second,
	}
	c, err := dialer.Dial("tcp4", remoteAddr)
	return c, lb, idx, err
}