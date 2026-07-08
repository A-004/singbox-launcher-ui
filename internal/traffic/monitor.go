package traffic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/pion/stun"
)

const reconnectDelay = 3 * time.Second
const stunDefaultServer = "stun.l.google.com:19302"
const geoServiceURL = "https://am.i.mullvad.net/json"
const geoPollInterval = 60 * time.Second

type connectionsResponse struct {
	DownloadTotal int64 `json:"downloadTotal"`
	UploadTotal   int64 `json:"uploadTotal"`
}

// Monitor polls the Clash API /connections endpoint every 2 seconds,
// resolves external IP via STUN, measures TCP ping delay every 3 seconds,
// and fetches geo-location every 60s — all independently so a failure
// in one doesn't block the others.
type Monitor struct {
	mu      sync.Mutex
	cfg     ClashConfig
	client  *http.Client
	stopCh  chan struct{}
	statsCh chan TrafficStats
	running bool

	prevDownload int64
	prevUpload   int64
	prevTime     time.Time
	seeded       bool

	stunServer  string
	serverIP    string // VPN server IP from STUN
	localIP     string // non-VPN interface IP for binding
	lastDelayMs int64
	pingInfo    string // diagnostic text for the UI

	lastGeo     GeoInfo
	lastGeoTime time.Time
}

func NewMonitor(cfg ClashConfig) *Monitor {
	m := &Monitor{
		cfg:         cfg,
		client:      &http.Client{Timeout: 4 * time.Second},
		statsCh:     make(chan TrafficStats, 8),
		stopCh:      make(chan struct{}),
		lastDelayMs: -1,
		stunServer:  stunDefaultServer,
	}

	// Try cfg.LocalAddr, then auto-detect
	if cfg.LocalAddr != "" {
		if ip := net.ParseIP(cfg.LocalAddr); ip != nil {
			m.localIP = cfg.LocalAddr
			m.pingInfo = "Bind: " + cfg.LocalAddr
		}
	}
	if m.localIP == "" {
		if ip := resolveNonVPNIP(); ip != "" {
			m.localIP = ip
			m.pingInfo = "Bind: " + ip
		} else {
			m.pingInfo = "No bind IP (direct route)"
		}
	}

	return m
}

func resolveNonVPNIP() string {
	names := []string{"Ethernet", "eth0", "en0", "enp0s3"}
	for _, name := range names {
		iface, err := net.InterfaceByName(name)
		if err != nil {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() {
				continue
			}
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				return ip4.String()
			}
		}
	}
	return ""
}

func (m *Monitor) Stats() <-chan TrafficStats {
	return m.statsCh
}

func (m *Monitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopCh = make(chan struct{})
	m.mu.Unlock()
	go m.pollLoop()
}

func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)
}

// fetchGeo получает геолокацию через Mullvad API (через VPN-туннель).
// Использует отдельный HTTP-клиент без привязки к интерфейсу, чтобы
// запрос гарантированно шёл через VPN-туннель (основной маршрут системы).
// При ошибке (или выключенном VPN) Geo остаётся пустым.
func (m *Monitor) fetchGeo() {
	geoClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := geoClient.Get(geoServiceURL)
	if err != nil {
		m.mu.Lock()
		m.pingInfo = "Geo err: " + err.Error()
		m.mu.Unlock()
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		m.mu.Lock()
		m.pingInfo = "Geo read err: " + err.Error()
		m.mu.Unlock()
		return
	}

	var mullvadResp struct {
		IP      string `json:"ip"`
		Country string `json:"country"`
		City    string `json:"city"`
	}
	if err := json.Unmarshal(body, &mullvadResp); err != nil {
		m.mu.Lock()
		m.pingInfo = "Geo JSON err: " + err.Error()
		m.mu.Unlock()
		return
	}

	m.mu.Lock()
	m.lastGeo = GeoInfo{Country: mullvadResp.Country, City: mullvadResp.City}
	m.lastGeoTime = time.Now()
	m.pingInfo = fmt.Sprintf("Geo: %s", m.lastGeo.String())
	m.mu.Unlock()
}

// --- internal ---

func (m *Monitor) pollLoop() {
	defer func() {
		m.mu.Lock()
		close(m.statsCh)
		m.mu.Unlock()
	}()

	// Initial measurements
	m.resolveServerIP()
	m.measurePing()
	m.fetchGeo()

	// Three independent timers:
	//   trafficTicker — fetch /connections every 2s
	//   pingTicker    — TCP ping every 3s
	//   stunAndGeoTicker — resolve IP + geo every 60s
	trafficTicker := time.NewTicker(2 * time.Second)
	pingTicker := time.NewTicker(3 * time.Second)
	stunAndGeoTicker := time.NewTicker(60 * time.Second)
	defer trafficTicker.Stop()
	defer pingTicker.Stop()
	defer stunAndGeoTicker.Stop()

	for {
		select {
		case <-trafficTicker.C:
			m.sample()

		case <-pingTicker.C:
			m.measurePing()

		case <-stunAndGeoTicker.C:
			m.resolveServerIP()
			m.fetchGeo()

		case <-m.stopCh:
			return
		}
	}
}

// resolveServerIP does STUN through VPN tunnel to get the server's external IP.
func (m *Monitor) resolveServerIP() {
	m.mu.Lock()
	oldIP := m.serverIP
	m.mu.Unlock()

	conn, err := net.Dial("udp", m.stunServer)
	if err != nil {
		m.mu.Lock()
		m.pingInfo = "STUN dial err: " + err.Error()
		m.mu.Unlock()
		return
	}
	defer conn.Close()

	c, err := stun.NewClient(conn)
	if err != nil {
		m.mu.Lock()
		m.pingInfo = "STUN client err: " + err.Error()
		m.mu.Unlock()
		return
	}
	defer c.Close()

	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	var xorAddr stun.XORMappedAddress
	var errResult error

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		err = c.Do(message, func(res stun.Event) {
			if res.Error != nil {
				errResult = res.Error
				return
			}
			if err := xorAddr.GetFrom(res.Message); err != nil {
				errResult = err
				return
			}
		})
		if err != nil {
			errResult = err
		}
		close(done)
	}()

	select {
	case <-done:
		if errResult != nil {
			m.mu.Lock()
			m.pingInfo = "STUN err: " + errResult.Error()
			m.mu.Unlock()
			return
		}
		ip := xorAddr.IP.String()
		if ip == "" {
			m.mu.Lock()
			m.pingInfo = "STUN: empty IP response"
			m.mu.Unlock()
			return
		}
		m.mu.Lock()
		m.serverIP = ip
		if oldIP != ip {
			m.pingInfo = fmt.Sprintf("STUN OK: %s (new)", ip)
		} else {
			m.pingInfo = fmt.Sprintf("STUN OK: %s", ip)
		}
		m.mu.Unlock()

	case <-ctx.Done():
		m.mu.Lock()
		m.pingInfo = "STUN: timeout (5s)"
		m.mu.Unlock()
	}
}

// measurePing does TCP connect. Returns true if a measurement was obtained
// (even if the port returned RST). Returns false if no IP or all ports timed out.
func (m *Monitor) measurePing() bool {
	m.mu.Lock()
	ip := m.serverIP
	bindIP := m.localIP
	m.mu.Unlock()

	if ip == "" {
		m.mu.Lock()
		m.pingInfo = "No server IP (STUN pending)"
		m.mu.Unlock()
		return false
	}

	d, info := tcpingDiagnostic(ip, bindIP, 2*time.Second)

	m.mu.Lock()
	m.lastDelayMs = d
	m.pingInfo = info
	m.mu.Unlock()
	return d > 0
}

// MeasureServerDelay returns RTT in ms, -1 on failure.
func (m *Monitor) MeasureServerDelay() int64 {
	m.mu.Lock()
	ip := m.serverIP
	bindIP := m.localIP
	m.mu.Unlock()
	if ip == "" {
		return -1
	}
	d, _ := tcpingDiagnostic(ip, bindIP, 2*time.Second)
	return d
}

func (m *Monitor) sample() {
	m.mu.Lock()
	serverIP := m.serverIP
	lastDelay := m.lastDelayMs
	localAddr := m.localIP
	diag := m.pingInfo
	geo := m.lastGeo
	m.mu.Unlock()

	curDl, curUl, err := m.fetchTotals()
	if err != nil {
		return
	}

	m.mu.Lock()

	if !m.seeded {
		m.seeded = true
		m.prevDownload = curDl
		m.prevUpload = curUl
		m.prevTime = time.Now()
		m.mu.Unlock()
		return
	}

	if curDl < m.prevDownload || curUl < m.prevUpload {
		m.prevDownload = curDl
		m.prevUpload = curUl
		m.prevTime = time.Now()
		m.seeded = false
		m.mu.Unlock()
		return
	}

	now := time.Now()
	elapsed := now.Sub(m.prevTime).Seconds()
	if elapsed <= 0 {
		elapsed = 1
	}

	dlBps := float64(curDl-m.prevDownload) / elapsed
	ulBps := float64(curUl-m.prevUpload) / elapsed
	if dlBps < 0 {
		dlBps = 0
	}
	if ulBps < 0 {
		ulBps = 0
	}

	m.prevDownload = curDl
	m.prevUpload = curUl
	m.prevTime = now

	pingOk := lastDelay > 0
	proxyAddr := serverIP
	if proxyAddr == "" {
		proxyAddr = "N/A"
	}

	bindLabel := localAddr
	if bindLabel == "" {
		bindLabel = "direct"
	}

	stats := NewTrafficStats(dlBps, ulBps, lastDelay, bindLabel, proxyAddr, pingOk, diag)
	stats.Geo = geo
	m.statsCh <- stats
	m.mu.Unlock()
}

func (m *Monitor) fetchTotals() (int64, int64, error) {
	req, err := http.NewRequest("GET", m.cfg.Addr()+"/connections", nil)
	if err != nil {
		return 0, 0, err
	}
	if m.cfg.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+m.cfg.Secret)
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, err
	}

	var cr connectionsResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return 0, 0, err
	}
	return cr.DownloadTotal, cr.UploadTotal, nil
}

// tcpingDiagnostic does TCP connect on multiple ports and returns (RTT_ms, diag_string).
// Each port is given at most perPortTimeout; total time is capped at ~len(ports)*perPortTimeout.
func tcpingDiagnostic(ip, bindIP string, timeout time.Duration) (int64, string) {
	ports := []int{443, 80, 22, 4500, 8080, 8443}
	// Per-port timeout: 500ms is enough to detect SYN-ACK or RST on typical links.
	// This prevents the whole monitor from freezing when the first few ports time out.
	perPortTimeout := 500 * time.Millisecond
	bestMs := int64(-1)
	bestInfo := ""

	var dialer net.Dialer
	bindNote := ""
	if bindIP != "" {
		if parsed := net.ParseIP(bindIP); parsed != nil {
			dialer.LocalAddr = &net.TCPAddr{IP: parsed}
			bindNote = " (bind " + bindIP + ")"
		}
	}

	for _, port := range ports {
		addr := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
		start := time.Now()
		conn, err := dialer.Dial("tcp", addr)
		elapsed := time.Since(start)
		ms := elapsed.Milliseconds()
		if ms < 1 && err == nil {
			ms = 1
		}

		if err == nil {
			conn.Close()
			info := fmt.Sprintf("TCP %d%s: SYN-ACK %dms", port, bindNote, ms)
			return ms, info
		}

		// Fast error = RST (port closed) = valid RTT
		if ms > 0 && ms < int64(perPortTimeout.Milliseconds()) {
			info := fmt.Sprintf("TCP %d%s: RST %dms", port, bindNote, ms)
			if bestMs < 0 || ms < bestMs {
				bestMs = ms
				bestInfo = info
			}
			continue
		}

		// Full timeout — try next port
		continue
	}

	if bestMs > 0 {
		return bestMs, bestInfo
	}
	return -1, fmt.Sprintf("TCP %s%s: all %d ports timeout", ip, bindNote, len(ports))
}
