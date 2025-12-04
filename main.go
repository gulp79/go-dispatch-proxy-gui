package main

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	gonet "github.com/shirou/gopsutil/v3/net"
)

var proxy ProxyServer

// Struttura per tracciare lo stato delle NIC nella GUI
type NICRow struct {
	Name     string
	IP       string
	Check    *widget.Check
	Slider   *widget.Slider
	ValueLbl *widget.Label

	// Widget per le statistiche (riutilizzati)
	StatsNameLbl *widget.Label
	UpLbl        *widget.Label
	DownLbl      *widget.Label
	Graph        *MiniGraph
	PrevSent     uint64
	PrevRecv     uint64
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC: %v\n", r)
		}
	}()

	a := app.NewWithID("com.dispatch.proxy")
	a.Settings().SetTheme(&MatrixTheme{})
	w := a.NewWindow("Go Dispatch Proxy - Unified")
	w.Resize(fyne.NewSize(1100, 700))

	// --- Left Panel Components ---
	hostEntry := widget.NewEntry()
	hostEntry.SetText("127.0.0.1")
	portEntry := widget.NewEntry()
	portEntry.SetText("8080")
	tunnelCheck := widget.NewCheck("Tunnel Mode", nil)
	
	// ✓ Quiet Mode: attivo di default per ridurre carico CPU/RAM
	quietCheck := widget.NewCheck("Quiet Mode (hide DEBUG logs)", nil)
	quietCheck.Checked = true
	
	// ✓ Enable Logs: permette di disabilitare completamente i log
	enableLogCheck := widget.NewCheck("Enable Logs", nil)
	enableLogCheck.Checked = true

	nicContainer := container.NewVBox()
	statsContainer := container.NewVBox()
	
	var nicRows = make(map[string]*NICRow)
	var nicMutex sync.RWMutex

	// Funzione per ricostruire l'interfaccia quando si fa "Refresh"
	refreshNICs := func() {
		nicMutex.Lock()
		defer nicMutex.Unlock()

		nicContainer.Objects = nil
		statsContainer.Objects = nil

		// Intestazione Statistiche (Fissa)
		headerObj := container.NewGridWithColumns(4,
			widget.NewLabelWithStyle("Interface", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Upload (Mb/s)", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Download (Mb/s)", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Activity", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		)
		statsContainer.Add(headerObj)

		loadedNICs := getValidInterfaces()
		sort.Slice(loadedNICs, func(i, j int) bool { return loadedNICs[i].ip < loadedNICs[j].ip })

		for _, nic := range loadedNICs {
			// --- Componenti Selezione (Sinistra) ---
			lbl := widget.NewLabel(fmt.Sprintf("%s (%s)", nic.ip, nic.name))
			chk := widget.NewCheck("", nil)
			sl := widget.NewSlider(1, 4)
			sl.Step = 1
			sl.Value = 1
			valLbl := widget.NewLabel("1")

			// Ripristina stato precedente se esiste
			if old, ok := nicRows[nic.ip]; ok {
				chk.Checked = old.Check.Checked
				sl.Value = old.Slider.Value
				valLbl.SetText(old.ValueLbl.Text)
			}

			sl.OnChanged = func(v float64) { valLbl.SetText(fmt.Sprintf("%d", int(v))) }

			// --- Componenti Statistiche (Destra) ---
			sName := widget.NewLabel(fmt.Sprintf("%s (%s)", nic.ip, nic.name))
			sName.Truncation = fyne.TextTruncateEllipsis
			
			sUp := widget.NewLabel("0.00")
			sUp.Alignment = fyne.TextAlignTrailing
			
			sDown := widget.NewLabel("0.00")
			sDown.Alignment = fyne.TextAlignTrailing

			gr := NewMiniGraph(theme.PrimaryColor())

			row := &NICRow{
				Name: nic.name, IP: nic.ip, Check: chk, Slider: sl, ValueLbl: valLbl,
				StatsNameLbl: sName, UpLbl: sUp, DownLbl: sDown, Graph: gr,
			}
			nicRows[nic.ip] = row

			// ✓ Layout CORRETTO per sinistra con wrap
			sliderContainer := container.NewHBox(widget.NewLabel("Weight:"), sl, valLbl)
			topRow := container.NewBorder(nil, nil, chk, sliderContainer, lbl)
			nicContainer.Add(topRow)

			// Aggiungi a UI Destra (Grid statica)
			statsRow := container.NewGridWithColumns(4,
				sName,
				sUp,
				sDown,
				container.NewPadded(gr),
			)
			statsContainer.Add(statsRow)
		}
		
		nicContainer.Refresh()
		statsContainer.Refresh()
	}

	refreshBtn := widget.NewButton("Refresh Interfaces", refreshNICs)
	statusLabel := widget.NewLabel("🔴 Proxy: Stopped")
	statusLabel.TextStyle = fyne.TextStyle{Bold: true}
	startBtn := widget.NewButton("Start Proxy", nil)

	// --- Log Area Ottimizzata ---
	logArea := widget.NewMultiLineEntry()
	logArea.TextStyle = fyne.TextStyle{Monospace: true}
	logArea.Wrapping = fyne.TextWrapBreak
	logArea.Disable()

	var logMutex sync.Mutex
	logBuffer := make([]string, 0, 100)
	const maxLogLines = 100

	logger := func(msg string) {
		// ✓ Exit immediato se log disabilitati (zero overhead)
		if !enableLogCheck.Checked {
			return
		}
		
		logMutex.Lock()
		defer logMutex.Unlock()

		if quietCheck.Checked && strings.Contains(msg, "[DEBUG]") {
			return
		}

		if len(logBuffer) >= maxLogLines {
			logBuffer = logBuffer[1:]
		}
		logBuffer = append(logBuffer, msg)

		finalText := strings.Join(logBuffer, "\n")
		logArea.SetText(finalText)
		logArea.CursorRow = len(logBuffer) - 1
		logArea.Refresh()
	}

	// ✓ Pulsante Clear Logs
	clearLogsBtn := widget.NewButton("Clear Logs", func() {
		logMutex.Lock()
		logBuffer = logBuffer[:0] // Reset buffer
		logArea.SetText("")
		logMutex.Unlock()
	})
	clearLogsBtn.Importance = widget.LowImportance

	// --- Loop Statistiche Ottimizzato ---
	updateStats := func() {
		nicMutex.RLock()
		defer nicMutex.RUnlock()

		counters, err := gonet.IOCounters(true)
		if err != nil {
			return
		}
		counterMap := make(map[string]gonet.IOCountersStat)
		for _, c := range counters {
			counterMap[c.Name] = c
		}

		for _, row := range nicRows {
			stat, exists := counterMap[row.Name]
			if !exists {
				continue
			}

			var upRate, downRate float64
			if row.PrevSent > 0 {
				elapsed := 1.0
				upRate = float64(stat.BytesSent-row.PrevSent) * 8 / 1_000_000 / elapsed
				downRate = float64(stat.BytesRecv-row.PrevRecv) * 8 / 1_000_000 / elapsed
			}
			
			row.PrevSent = stat.BytesSent
			row.PrevRecv = stat.BytesRecv

			row.UpLbl.SetText(fmt.Sprintf("%.2f", upRate))
			row.DownLbl.SetText(fmt.Sprintf("%.2f", downRate))
			row.Graph.AddValue(downRate + upRate)

			if row.Check.Checked {
				row.StatsNameLbl.TextStyle = fyne.TextStyle{Bold: true}
				row.StatsNameLbl.SetText(fmt.Sprintf("▶ %s", row.IP))
			} else {
				row.StatsNameLbl.TextStyle = fyne.TextStyle{Bold: false}
				row.StatsNameLbl.SetText(fmt.Sprintf("%s", row.IP))
			}
		}
	}

	stopStats := make(chan struct{})
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				updateStats()
			case <-stopStats:
				return
			}
		}
	}()

	// Start Logic
	startBtn.OnTapped = func() {
		if proxy.running {
			proxy.Stop()
			startBtn.SetText("Start Proxy")
			startBtn.Importance = widget.MediumImportance
			statusLabel.SetText("🔴 Proxy: Stopped")
			return
		}

		nicMutex.RLock()
		var selected []string
		for ip, row := range nicRows {
			if row.Check.Checked {
				w := int(row.Slider.Value)
				if w > 1 {
					selected = append(selected, fmt.Sprintf("%s@%d", ip, w))
				} else {
					selected = append(selected, ip)
				}
			}
		}
		nicMutex.RUnlock()

		if len(selected) == 0 {
			dialog.ShowInformation("Error", "Please select at least one interface", w)
			return
		}

		port, err := strconv.Atoi(portEntry.Text)
		if err != nil {
			dialog.ShowError(fmt.Errorf("invalid port: %v", err), w)
			return
		}

		logger("--- Starting Proxy ---")
		go func() {
			err = proxy.Start(hostEntry.Text, port, tunnelCheck.Checked, selected, logger)
			if err != nil {
				logger(fmt.Sprintf("[ERROR] %v", err))
				statusLabel.SetText("🔴 Proxy: Error")
			}
		}()
		
		startBtn.SetText("Stop Proxy")
		startBtn.Importance = widget.HighImportance
		statusLabel.SetText("▶ Proxy: Running")
	}

	w.SetOnClosed(func() {
		close(stopStats)
		if proxy.running {
			proxy.Stop()
		}
	})

	// Init
	refreshNICs()

	// --- Layout Principale ---
	
	// Settings in alto a sinistra
	topSettings := container.NewVBox(
		widget.NewLabelWithStyle("Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewForm(
			widget.NewFormItem("Host", hostEntry),
			widget.NewFormItem("Port", portEntry),
		),
		tunnelCheck,
		quietCheck,
		enableLogCheck, // ✓ Checkbox per disabilitare log
		widget.NewSeparator(),
		container.NewHBox(
			widget.NewLabelWithStyle("Interfaces", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
			refreshBtn,
		),
	)

	bottomControls := container.NewVBox(
		widget.NewSeparator(),
		statusLabel,
		startBtn,
	)

	// ✓ Lista scrollabile interfacce (CORRETTO)
	nicScroll := container.NewVScroll(nicContainer)
	nicScroll.SetMinSize(fyne.NewSize(280, 0)) // ✓ Larghezza minima per vedere tutte le interfacce

	leftPanel := container.NewBorder(topSettings, bottomControls, nil, nil, nicScroll)

	// ✓ Right Panel con Clear Logs button
	logHeader := container.NewBorder(nil, nil,
		widget.NewLabelWithStyle("Logs", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		clearLogsBtn,
	)
	
	rightPanel := container.NewVSplit(
		container.NewBorder(logHeader, nil, nil, nil, logArea),
		container.NewBorder(
			widget.NewLabelWithStyle("Real-time Statistics", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			nil, nil, nil,
			container.NewVScroll(statsContainer),
		),
	)
	rightPanel.SetOffset(0.5)

	content := container.NewBorder(nil, nil, container.NewPadded(leftPanel), nil, rightPanel)
	w.SetContent(content)
	w.ShowAndRun()
}

type nicInfo struct {
	ip, name string
}

func getValidInterfaces() []nicInfo {
	var res []nicInfo
	ifaces, err := net.Interfaces()
	if err != nil {
		return res
	}

	// ✓ Filtro interfacce virtuali migliorato
	virtualPatterns := []string{
		"virtual", "vbox", "vmware", "vethernet", "veth",
		"docker", "vpn", "tap", "tun", "host-only",
	}

	for _, i := range ifaces {
		lowerName := strings.ToLower(i.Name)
		isVirtual := false
		for _, pattern := range virtualPatterns {
			if strings.Contains(lowerName, pattern) {
				isVirtual = true
				break
			}
		}
		if isVirtual {
			continue
		}

		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := i.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip string
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP.String()
			case *net.IPAddr:
				ip = v.IP.String()
			}

			// ✓ Filtra IP VirtualBox (192.168.56.*) e altri range locali
			if strings.Count(ip, ".") == 3 &&
				!strings.HasPrefix(ip, "127.") &&
				!strings.HasPrefix(ip, "169.254.") &&
				!strings.HasPrefix(ip, "192.168.56.") {
				res = append(res, nicInfo{ip, i.Name})
			}
		}
	}
	return res
}

