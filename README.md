# Go Dispatch Proxy GUI Enhanced

Go Dispatch Proxy GUI Enhanced is a Go/Fyne desktop application for sharing outbound TCP connections across multiple network interfaces. It is useful when several mobile tethering links, Wi-Fi adapters, or other independent connections are available and a download manager can open many parallel connections.

The proxy listens on one local port and automatically accepts:

- HTTP proxy requests, including `CONNECT` for HTTPS
- SOCKS5
- SOCKS4
- SOCKS4a
- Tunnel mode for forwarding all accepted connections to configured backend targets

## Features

- One GUI application written in Go.
- HTTP, SOCKS5, SOCKS4, and SOCKS4a on the same listening port.
- Weighted round-robin dispatch across selected local interfaces.
- Real-time upload/download statistics and small activity graphs per interface.
- Interface filtering to hide common virtual adapters.
- Optional quiet logs to reduce GUI overhead.
- Linux AppImage build script for a mostly self-contained executable.

## How It Works

This is connection-level load balancing, not true MPTCP bonding. Each TCP connection is assigned to one selected outgoing interface. Tools that open many parallel connections, such as JDownloader, IDM, aria2, or some browser/download-manager combinations, can therefore use several links at once.

A single browser download often uses only one TCP connection, so that individual download may not combine all interfaces by itself. It can still run at the same time as a download manager using the same proxy port.

## Quick Start

1. Connect the network devices you want to use.
2. Run the application.
3. Click `Refresh Interfaces`.
4. Select one or more interfaces.
5. Adjust each interface weight if needed.
6. Click `Start Proxy`.
7. Configure your browser or download manager to use the local proxy.

Default proxy settings:

- Host: `127.0.0.1`
- Port: `8080`
- Protocol: `HTTP`, `SOCKS5`, `SOCKS4`, or `SOCKS4a`

Different applications can use different protocols on the same host and port at the same time. The proxy detects the protocol independently for each incoming connection.

## Run on Linux

Use the prebuilt AppImage from this repository:

```bash
chmod +x build/Go_Dispatch_Proxy_GUI-x86_64.AppImage
./build/Go_Dispatch_Proxy_GUI-x86_64.AppImage
```

The AppImage bundles the application and the X11 libraries needed by Fyne/GLFW. It still requires a normal Linux graphical session and system OpenGL/graphics drivers.

## Build From Source

Requirements:

- Go 1.24+ or the toolchain from `go.mod`
- A C compiler
- Fyne/GLFW development dependencies for native builds

Run directly:

```bash
go mod tidy
go run .
```

Build a native binary:

```bash
go build -ldflags="-s -w" -o dist/dispatch-proxy-gui .
```

## Build AppImage

The AppImage build script downloads the required Ubuntu X11 development packages into a local `build/sysroot` directory. It does not install system packages with `sudo`.

```bash
./scripts/build-appimage.sh
```

Output:

```bash
build/Go_Dispatch_Proxy_GUI-x86_64.AppImage
```

## Notes

- AppImage packaging currently targets `x86_64` Linux.
- On Linux, interface binding uses `BindToDevice` when an interface name can be resolved.
- On Windows and macOS, the app binds by local IP address where supported by the OS network stack.
- No proxy authentication is implemented.

## Credits

This project is inspired by the original `go-dispatch-proxy` work by `extremecoders-re` and the GUI fork this enhanced version was based on.
