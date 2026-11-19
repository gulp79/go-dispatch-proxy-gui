# Istruzioni per Agenti AI - Go Dispatch Proxy GUI

Questo documento definisce le linee guida operative, i vincoli architetturali e le regole di sviluppo per gli agenti AI che lavorano sulla codebase di **Go Dispatch Proxy GUI**.

---

## 1. Panoramica del Progetto

**Go Dispatch Proxy GUI** è un'applicazione standalone scritta in **Go** con interfaccia grafica **Fyne v2** (`fyne.io/fyne/v2`).
Lo scopo primario è l'aggregazione di banda da connessioni di rete multiple (es. tethering USB/Wi-Fi di più smartphone 4G/5G, schede di rete multiple) tramite un proxy locale SOCKS5 che effettua il bilanciamento delle connessioni uscenti con algoritmo **Weighted Round-Robin**.

### Componenti Principali

* `main.go`: Entry point dell'applicazione, inizializzazione del toolkit grafico Fyne, monitoraggio dello stato delle schede di rete (NIC) tramite `gopsutil/v4/net`, gestione dei controlli utente e ciclo di vita del server.
* `proxy_backend.go`: Motore logico del proxy; gestisce il listener TCP, il `Dispatcher`, la coda dei backend con pesi (`ContentionRatio`) e la sincronizzazione delle connessioni attive (`sync.WaitGroup`, `sync.Mutex`).
* `socks_proto.go`: Implementazione del protocollo SOCKS5 RFC 1928 (handshake no-auth, parsing comandi CONNECT per IPv4, Domain name e IPv6, fallback in caso di fallimento del dialer).
* `ui_components.go`: Widget personalizzati Fyne, in particolare `MiniGraph` con `graphRenderer` dedicato per il rendering in tempo reale dei grafici di traffico (upload/download).
* `theme.go`: Definizione del tema grafico (`MatrixTheme`) e palette colori Fyne.
* `dialer_linux.go` / `dialer_others.go`: Dialer di rete con build tags specifici per sistema operativo:
  * Su Linux (`//go:build linux`): binding al dispositivo tramite syscall `SO_BINDTODEVICE`.
  * Sugli altri OS (`//go:build !linux`): binding all'indirizzo IP locale via `net.Dialer.LocalAddr`.
* `FyneApp.toml`: Metadati Fyne (App ID `com.gulp79.dispatchproxy`, nome, versione e icona).
* `.github/workflows/build.yml`: Pipeline CI/CD multipiattaforma per la compilazione, packaging e rilascio.

---

## 2. Regola Fondamentale di Compilazione (STRICT)

> ⚠️ **LA COMPILAZIONE AVVIENE ESCLUSIVAMENTE SU GITHUB ACTIONS, MAI IN LOCALE.**

1. **Nessuna Build Locale**:

   * **NON** eseguire `go build`, `go run`, `fyne package` o comandi `fyne-cross` sulla macchina locale.
   * La macchina di sviluppo locale non richiede né deve configurare toolchain C (GCC/MinGW), dipendenze grafiche OpenGL o container Docker locali per la compilazione cross-platform.

2. **Workflow CI/CD Dedicato**:

   * Tutto il processo di build, cross-compilazione (Windows `amd64`, `386`, `arm64`; Linux `amd64`, `386`, `arm64`, `arm`) e generazione degli artefatti è delegato a GitHub Actions (`.github/workflows/build.yml`).
   * Il workflow si attiva tramite:
     * Trigger manuale (`workflow_dispatch`).
     * Push di tag di release (es. `v1.0.1`).

3. **Verifiche Consentite in Locale**:

   * È possibile eseguire solo analisi statica leggera e verifica di sintassi non compilativa, ad esempio:
     * `go vet ./...` (verifica coerenza statica Go).
     * `git status`, `git diff` (controllo modifiche).
   * Se si manipola `go.mod`, è consentito `go mod tidy` solo se necessario per allineare le dipendenze, assicurandosi che restino compatibili con i container di build CI.

   Commenti e testi vanno inseriti e composti in lingua inglese.

---

## 3. Standard e Pattern di Sviluppo

### Thread Safety e Concorrenza (Fyne & Goroutine)

* Fyne gestisce il loop grafico nel thread principale. Qualsiasi aggiornamento della UI generato da goroutine di background (come le statistiche di banda o i log inviati da `proxy_backend.go` / `socks_proto.go`) deve essere rigorosamente thread-safe.
* Per il refresh della grafica o aggiornamenti canvas, utilizzare `fyne.Do(func() { ... })` o aggiornare campi protetti da mutex.
* Non bloccare mai il thread grafico: operazioni di I/O di rete devono avvenire sempre in goroutine separate.

### Mantenimento dei Dialer OS-Specifici

* Quando si modifica la firma o la logica di `DialBackend`:
  * Aggiornare contemporaneamente **sia** `dialer_linux.go` **che** `dialer_others.go`.
  * La firma della funzione deve rimanere perfettamente identica:
    ```go
    func DialBackend(d *Dispatcher, remoteAddr string) (net.Conn, *Backend, int, error)
    ```
  * Mantenere intatti i build tags in cima ai file (`//go:build linux` e `//go:build !linux`).

### Prestazioni e Logging

* Durante i download con download manager multi-connessione (JDownloader, IDM, aria2), il proxy gestisce centinaia di connessioni concorrenti con flussi `io.Copy`.
* Non inserire log sincroni o stampe a schermo pesanti all'interno dei loop di trasferimento dati (`io.Copy`), per non degradare il throughput e non saturare la memoria con allocazioni di stringhe.
* Rispettare i flag `Quiet Mode` e `Enable Logs` gestiti in `main.go`.

### Versioning e Rilasci

* Prima di un nuovo rilascio:
  1. Aggiornare `Version` e `Build` in `FyneApp.toml`.
  2. Aggiornare le note di rilascio o il `README.md` se sono cambiate funzionalità visibili all'utente.
  3. Il rilascio effettivo avviene creando ed inviando il tag Git corrispondente (es. `v1.0.1`), che scatenerà la creazione della GitHub Release con i binari zippati.

---

## 4. Linee Guida per gli Agenti AI

1. **Ispezione prima della modifica**: Leggere sempre i file rilevanti prima di applicare modifiche.
2. **Modifiche minime e chirurgiche**: Preferire sostituzioni mirate e preservare la formattazione esistente.
3. **Nessun push o tag non autorizzato**: Non eseguire `git push`, non creare tag Git remoti e non pubblicare release senza l'esplicita conferma dell'utente.
4. **Rispetto delle regole globali**: Questo file integra e rispetta le direttive generali dell'ambiente OpenCode (nessuna azione distruttiva non reversibile).
