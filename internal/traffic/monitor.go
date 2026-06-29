package traffic

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
)

// reconnectDelay is the wait between retry attempts when the API is down.
const reconnectDelay = 3 * time.Second

// connectionsResponse is the JSON response from GET /connections.
// We only need the cumulative totals for speed calculation.
type connectionsResponse struct {
	DownloadTotal int64 `json:"downloadTotal"`
	UploadTotal   int64 `json:"uploadTotal"`
}

// Monitor polls the Clash API /connections endpoint every second,
// computes download/upload speeds from cumulative byte totals,
// and delivers TrafficStats through a channel.
// Auto-reconnects every 3s on error.
type Monitor struct {
	mu      sync.Mutex
	cfg     ClashConfig
	client  *http.Client
	ticker  *time.Ticker
	stopCh  chan struct{}
	statsCh chan TrafficStats
	running bool

	// previous snapshot for speed calculation
	prevDownload int64
	prevUpload   int64
	prevTime     time.Time
}

// NewMonitor creates a stopped Monitor. Call Start() to begin polling.
func NewMonitor(cfg ClashConfig) *Monitor {
	return &Monitor{
		cfg:     cfg,
		client:  &http.Client{Timeout: 4 * time.Second},
		statsCh: make(chan TrafficStats, 8),
		stopCh:  make(chan struct{}),
	}
}

// Stats returns a read-only channel of traffic snapshots (one per second).
func (m *Monitor) Stats() <-chan TrafficStats {
	return m.statsCh
}

// Start begins polling the Clash API /connections every second.
func (m *Monitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopCh = make(chan struct{})
	m.ticker = time.NewTicker(1 * time.Second)
	m.mu.Unlock()

	// Seed with initial cumulative totals.
	if dl, ul, err := m.fetchTotals(); err == nil {
		m.mu.Lock()
		m.prevDownload = dl
		m.prevUpload = ul
		m.prevTime = time.Now()
		m.mu.Unlock()
	}

	go m.pollLoop()
}

// Stop terminates the poll loop and closes the stats channel.
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	if m.ticker != nil {
		m.ticker.Stop()
	}
	close(m.stopCh)
}

func (m *Monitor) pollLoop() {
	defer func() {
		m.mu.Lock()
		close(m.statsCh)
		m.mu.Unlock()
	}()

	for {
		select {
		case <-m.ticker.C:
			m.sample()
		case <-m.stopCh:
			return
		}
	}
}

func (m *Monitor) sample() {
	curDl, curUl, err := m.fetchTotals()
	if err != nil {
		// API not ready — don't send zeros, don't sleep. Just skip
		// this sample and let the next tick (1s) retry naturally.
		// Previously we pushed (0, 0) and blocked the loop for 3s,
		// which caused connected → zeros → connected flicker when
		// the API briefly stutters.
		return
	}

	m.mu.Lock()
	now := time.Now()
	elapsed := now.Sub(m.prevTime).Seconds()
	if elapsed <= 0 {
		elapsed = 1
	}

	dlBps := float64(curDl-m.prevDownload) / elapsed
	ulBps := float64(curUl-m.prevUpload) / elapsed
	if dlBps < 0 {
		dlBps = 0 // counters reset (sing-box restart)
	}
	if ulBps < 0 {
		ulBps = 0
	}

	m.prevDownload = curDl
	m.prevUpload = curUl
	m.prevTime = now
	m.mu.Unlock()

	m.statsCh <- NewTrafficStats(dlBps, ulBps)
}

// fetchTotals calls GET /connections and returns cumulative (downloadTotal, uploadTotal).
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
