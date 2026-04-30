package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

func (s *ProxyServer) handleHTTPProxy(conn net.Conn, reader *bufio.Reader) {
	for {
		req, err := http.ReadRequest(reader)
		if err != nil {
			if err != io.EOF {
				s.log(fmt.Sprintf("[WARN] HTTP request parse fail from %s: %v", conn.RemoteAddr(), err))
			}
			return
		}

		if req.Method == http.MethodConnect {
			s.handleHTTPConnect(conn, reader, req)
			return
		}

		closeClient, err := s.forwardHTTPRequest(conn, req)
		if err != nil {
			s.log(fmt.Sprintf("[WARN] HTTP forward fail from %s: %v", conn.RemoteAddr(), err))
			return
		}

		if req.Close || closeClient {
			return
		}
	}
}

func (s *ProxyServer) handleHTTPConnect(conn net.Conn, reader *bufio.Reader, req *http.Request) {
	dest, err := httpRequestDestination(req, "443")
	if err != nil {
		writeHTTPError(conn, http.StatusBadRequest)
		return
	}

	remote, lb, idx, err := DialBackend(s.dispatcher, dest)
	if err != nil {
		s.log(fmt.Sprintf("[WARN] HTTP CONNECT fail %s: %v", dest, err))
		writeHTTPError(conn, http.StatusBadGateway)
		return
	}

	s.log(fmt.Sprintf("[DEBUG] HTTP CONNECT %s -> %s (via %s LB:%d)", conn.RemoteAddr(), dest, lb.Address, idx))
	if _, err := conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		remote.Close()
		return
	}
	if err := flushBufferedToRemote(reader, remote); err != nil {
		remote.Close()
		return
	}
	pipe(conn, remote)
}

func (s *ProxyServer) forwardHTTPRequest(conn net.Conn, req *http.Request) (bool, error) {
	dest, err := httpRequestDestination(req, "80")
	if err != nil {
		writeHTTPError(conn, http.StatusBadRequest)
		return true, err
	}

	remote, lb, idx, err := DialBackend(s.dispatcher, dest)
	if err != nil {
		s.log(fmt.Sprintf("[WARN] HTTP connect fail %s: %v", dest, err))
		writeHTTPError(conn, http.StatusBadGateway)
		return true, err
	}
	defer remote.Close()

	s.log(fmt.Sprintf("[DEBUG] HTTP %s %s -> %s (via %s LB:%d)", req.Method, conn.RemoteAddr(), dest, lb.Address, idx))
	prepareProxyRequest(req)
	if err := req.Write(remote); err != nil {
		return true, err
	}

	remoteReader := bufio.NewReader(remote)
	resp, err := http.ReadResponse(remoteReader, req)
	if err != nil {
		writeHTTPError(conn, http.StatusBadGateway)
		return true, err
	}
	defer resp.Body.Close()

	if err := resp.Write(conn); err != nil {
		return true, err
	}
	return resp.Close, nil
}

func httpRequestDestination(req *http.Request, defaultPort string) (string, error) {
	host := req.Host
	if req.URL != nil && req.URL.Host != "" {
		host = req.URL.Host
		if req.URL.Scheme == "https" && defaultPort == "80" {
			defaultPort = "443"
		}
	}
	if host == "" {
		return "", fmt.Errorf("missing HTTP host")
	}
	return hostWithDefaultPort(host, defaultPort)
}

func hostWithDefaultPort(host string, defaultPort string) (string, error) {
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host, nil
	}

	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return net.JoinHostPort(strings.Trim(host, "[]"), defaultPort), nil
	}

	if strings.Count(host, ":") > 1 {
		return net.JoinHostPort(host, defaultPort), nil
	}

	if strings.Contains(host, ":") {
		hostOnly, port, ok := strings.Cut(host, ":")
		if ok && hostOnly != "" && port != "" {
			return net.JoinHostPort(hostOnly, port), nil
		}
		return "", fmt.Errorf("invalid HTTP host %q", host)
	}

	return net.JoinHostPort(host, defaultPort), nil
}

func prepareProxyRequest(req *http.Request) {
	req.RequestURI = ""
	req.Header.Del("Proxy-Authorization")
	req.Header.Del("Proxy-Connection")

	if req.URL == nil {
		return
	}
	if req.URL.Path == "" {
		req.URL.Path = "/"
	}
	req.URL.Scheme = ""
	req.URL.Host = ""
}

func writeHTTPError(conn net.Conn, status int) {
	text := http.StatusText(status)
	if text == "" {
		text = "Proxy Error"
	}
	body := text + "\n"
	fmt.Fprintf(conn, "HTTP/1.1 %d %s\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", status, text, len(body), body)
}
