package main

import (
	"fmt"
	"io"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"
	// "time" rimossa perché non usata
)

// LoggerFunc definisce come inviare i log alla GUI
type LoggerFunc func(string)

// ProxyServer gestisce lo stato del server
type ProxyServer struct {
	listener    net.Listener
	running     bool
	stopChan    chan struct{}
	dispatcher  *Dispatcher
	log         LoggerFunc
	mu          sync.Mutex
	activeConns sync.WaitGroup
}

// Backend rappresenta un'interfaccia di uscita
type Backend struct {
	Address            string
	Interface          string
	ContentionRatio    int
	CurrentConnections int
}

// Dispatcher gestisce il round-robin pesato
type Dispatcher struct {
	backends []*Backend
	mu       sync.Mutex
	index    int
}

func NewDispatcher(backends []*Backend) *Dispatcher {
	return &Dispatcher{backends: backends, index: 0}
}

func (d *Dispatcher) Next() (*Backend, int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.backends) == 0 {
		return nil, -1
	}
	lb := d.backends[d.index]
	idx := d.index
	lb.CurrentConnections++
	if lb.CurrentConnections >= lb.ContentionRatio {
		lb.CurrentConnections = 0
		d.index = (d.index + 1) % len(d.backends)
	}
	return lb, idx
}

func (d *Dispatcher) GetNextFailed(failedIndices *big.Int) (*Backend, int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := 0; i < len(d.backends); i++ {
		idx := (d.index + i) % len(d.backends)
		if failedIndices.Bit(idx) == 0 {
			return d.backends[idx], idx
		}
	}
	return nil, -1
}

// Start avvia il proxy
func (s *ProxyServer) Start(lhost string, lport int, tunnelMode bool, backendsConf []string, logger LoggerFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server already running")
	}

	s.log = logger
	backends := parseLoadBalancers(backendsConf, tunnelMode)
	if len(backends) == 0 {
		return fmt.Errorf("no backends selected")
	}

	s.dispatcher = NewDispatcher(backends)
	bindAddr := fmt.Sprintf("%s:%d", lhost, lport)
	l, err := net.Listen("tcp4", bindAddr)
	if err != nil {
		return err
	}

	s.listener = l
	s.running = true
	s.stopChan = make(chan struct{})

	s.log(fmt.Sprintf("[INFO] Server started on %s (Mode: Tunnel=%v)", bindAddr, tunnelMode))

	go s.acceptLoop(tunnelMode)
	return nil
}

func (s *ProxyServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	close(s.stopChan)
	if s.listener != nil {
		s.listener.Close()
	}
	s.log("[INFO] Server stopped, waiting for connections to drain...")
}

func (s *ProxyServer) acceptLoop(tunnel bool) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopChan:
				return // Stop normale
			default:
				s.log(fmt.Sprintf("[ERR] Accept: %v", err))
				continue
			}
		}

		s.activeConns.Add(1)
		go func(c net.Conn) {
			defer s.activeConns.Done()
			if tunnel {
				s.handleTunnel(c)
			} else {
				s.handleSocks(c)
			}
		}(conn)
	}
}

func parseLoadBalancers(args []string, isTunnel bool) []*Backend {
	list := make([]*Backend, 0, len(args))
	for _, arg := range args {
		parts := strings.Split(arg, "@")
		addrPart := parts[0]
		ratio := 1
		if len(parts) > 1 {
			if r, err := strconv.Atoi(parts[1]); err == nil && r > 0 {
				ratio = r
			}
		}

		var fullAddr, iface string
		if isTunnel {
			host, portStr, _ := net.SplitHostPort(addrPart)
			p, _ := strconv.Atoi(portStr)
			fullAddr = fmt.Sprintf("%s:%d", host, p)
		} else {
			if net.ParseIP(addrPart) == nil {
				continue
			}
			fullAddr = fmt.Sprintf("%s:0", addrPart)
			iface = getInterfaceFromIP(addrPart)
		}

		list = append(list, &Backend{
			Address:         fullAddr,
			Interface:       iface,
			ContentionRatio: ratio,
		})
	}
	return list
}

func getInterfaceFromIP(ip string) string {
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.String() == ip {
					return iface.Name
				}
			}
		}
	}
	return ""
}

func pipe(local, remote net.Conn) {
	done := make(chan struct{}, 2)
	cp := func(dst, src net.Conn) {
		io.Copy(dst, src)
		if c, ok := dst.(*net.TCPConn); ok {
			c.CloseWrite()
		}
		done <- struct{}{}
	}
	go cp(local, remote)
	go cp(remote, local)
	<-done
	local.Close()
	remote.Close()
}
