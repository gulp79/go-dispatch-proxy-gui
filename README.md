# Go Dispatch Proxy (Unified) 🚀
[![GitHub release](https://img.shields.io/github/v/release/gulp79/go-dispatch-proxy-gui?include_prereleases)](https://github.com/gulp79/go-dispatch-proxy-gui/releases/latest) ![Latest Downloads](https://img.shields.io/github/downloads/gulp79/go-dispatch-proxy-gui/total)

<img width="1280" height="764" alt="aaaaaa" src="https://github.com/user-attachments/assets/fb921e35-2ef7-48d1-8bee-e6a107bb5e36" />


A high-performance, unified SOCKS5 proxy application written in Go and Fyne (for the GUI). This tool is designed to solve a common connectivity problem: **aggregating bandwidth from multiple independent network interfaces**, particularly mobile tethering connections, to achieve higher overall throughput.

Ideal for users lacking high-speed fiber or ADSL connections who need to combine the speed of several 4G/5G phones for large downloads.

---

## ✨ Features

* **Unified Application:** The proxy backend and the graphical user interface (GUI) are compiled into a **single, standalone executable**. No Python, no dependencies, just Go performance.
* **Weighted Load Balancing (SOCKS5):** Distributes incoming TCP connections across multiple local IP addresses (your connected phones) using a customizable **Weighted Round Robin** algorithm.
* **Real-time Statistics:** Visual feedback on the bandwidth usage of each connected interface, including mini-graphs, to monitor performance and identify bottlenecks.
* **Network Filtering:** Automatically filters out virtual interfaces (like VirtualBox, VMware, Loopback, etc.) to keep the selection list clean and focused on actual internet sources.
* **High Performance:** Built entirely in Go for low CPU usage, minimal memory footprint, and high concurrency, crucial for managing hundreds of parallel connections from modern download managers.
* **Cross-Platform:** Tested and built for Windows, macOS, and Linux (requires OS-specific network stack support for binding).

---

## 💻 How It Works

The proxy works on the principle of **Connection Load Balancing**, not true MPTCP bonding (which requires remote server support).

1.  **Preparation:** Connect multiple mobile devices (via USB tethering or Wi-Fi hotspot) to your computer. Each device provides a unique IP address (e.g., `192.168.42.10`, `172.20.10.2`, etc.).
2.  **Proxy Activation:** The **Go Dispatch Proxy** runs on your PC (e.g., `127.0.0.1:8080`).
3.  **Download Manager:** You must use a multi-threaded download manager (like **JDownloader 2, IDM, or aria2**) and configure it to use the proxy.
4.  **Aggregation:** When the Download Manager splits a large file into 30 chunks, the proxy distributes these 30 individual connections across your 3-4 available mobile connections, effectively summing their bandwidths.

---

## ⚙️ Usage & Configuration

### 1. Download and Run

Download the latest executable from the [Releases page](LINK_TO_YOUR_RELEASES). No installation is required.

### 2. Configure Interfaces

1.  Connect your mobile phones and ensure they are visible as network interfaces (NICs) on your PC.
2.  Start the `Go Dispatch Proxy` executable.
3.  Click **"Refresh Interfaces"** to load the list of available connections.
4.  Select the interfaces you wish to use by checking the boxes.
5.  **Set Weight (Optional):** Use the slider next to each interface to set its weight (default is 1). An interface with weight **2** will receive twice as many connections as an interface with weight **1**. Use this to prioritize faster connections.
6.  Click **"Start Proxy"**.

### 3. Configure Download Manager

Set your download manager or web browser to use the SOCKS5 proxy running on:

* **Host:** `127.0.0.1`
* **Port:** `8080` (or the port you configured in the app)
* **Protocol:** **SOCKS5** (Crucial: not HTTP/HTTPS)

**Ensure your download manager is configured to use the maximum number of parallel connections (e.g., 16-32) per file to achieve full aggregation.**

---

## 🛠️ Building from Source

This project uses Go with the Fyne toolkit for the GUI.

### Prerequisites

* [Go 1.21+](https://go.dev/dl/)

### Build Steps

```bash
# Clone the repository
git clone [YOUR_REPO_URL]
cd [REPO_NAME]

# Fetch Fyne dependencies
go mod tidy

# Build the unified executable for your system
# The output file will be in the 'dist' folder.
go install fyne.io/fyne/v2/cmd/fyne@latest
fyne package -os windows/amd64 -icon icon.png -name "Go Dispatch Proxy"
# or simply:
go build -ldflags="-s -w" -o dist/dispatch-proxy.exe .
