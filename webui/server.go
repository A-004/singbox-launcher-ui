package webui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"sync"

	"singbox-launcher/api"
	"singbox-launcher/core"
	"singbox-launcher/core/services"
	"singbox-launcher/internal/debuglog"
)

//go:embed dashboard.html dashboard.css dashboard.js
var webUI embed.FS

// WebDashboard представляет HTTP-сервер для веб-интерфейса.
type WebDashboard struct {
	server  *http.Server
	port    int
	ctrl    *core.AppController
	mu      sync.Mutex
	running bool
}

// ProxyItem — данные о прокси для JSON-ответа.
type ProxyItem struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Delay     int64  `json:"delay"`
	DelayText string `json:"delayText"`
	DelayOK   bool   `json:"delayOk"`
	Selected  bool   `json:"selected"`
}

// StatusResponse — JSON ответ для /api/status
type StatusResponse struct {
	VPNRunning  bool        `json:"vpnRunning"`
	CoreVersion string      `json:"coreVersion"`
	ProxyCount  int         `json:"proxyCount"`
	Selected    string      `json:"selected"`
	Proxies     []ProxyItem `json:"proxies"`
}

// NewWebDashboard создаёт новый веб-дашборд.
func NewWebDashboard(ctrl *core.AppController, port int) *WebDashboard {
	return &WebDashboard{
		port: port,
		ctrl: ctrl,
	}
}

// Start запускает HTTP-сервер в фоновой горутине.
func (wd *WebDashboard) Start() error {
	wd.mu.Lock()
	defer wd.mu.Unlock()

	if wd.running {
		return nil
	}

	// Встраиваем статические файлы
	subFS, err := fs.Sub(webUI, ".")
	if err != nil {
		return fmt.Errorf("webui: failed to get sub fs: %w", err)
	}
	fileServer := http.FileServer(http.FS(subFS))

	mux := http.NewServeMux()
	mux.Handle("/", fileServer)
	mux.HandleFunc("/api/status", wd.handleStatus)
	mux.HandleFunc("/api/proxies", wd.handleProxies)
	mux.HandleFunc("/api/toggle-vpn", wd.handleToggleVPN)
	mux.HandleFunc("/api/select-proxy", wd.handleSelectProxy)

	addr := fmt.Sprintf("127.0.0.1:%d", wd.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("webui: failed to listen on %s: %w", addr, err)
	}

	wd.server = &http.Server{Handler: mux}
	wd.running = true

	go func() {
		debuglog.InfoLog("Web Dashboard: listening on http://%s", addr)
		if err := wd.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			debuglog.ErrorLog("Web Dashboard: serve error: %v", err)
		}
	}()

	return nil
}

// Stop останавливает HTTP-сервер.
func (wd *WebDashboard) Stop() {
	wd.mu.Lock()
	defer wd.mu.Unlock()

	if wd.server != nil {
		wd.server.Shutdown(context.Background())
		wd.running = false
		debuglog.InfoLog("Web Dashboard: stopped")
	}
}

// Port возвращает порт, на котором слушает сервер.
func (wd *WebDashboard) Port() int {
	return wd.port
}

// URL возвращает полный URL для открытия в браузере.
func (wd *WebDashboard) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", wd.port)
}

// handleStatus отдаёт статус приложения в JSON.
func (wd *WebDashboard) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	resp := StatusResponse{}

	if wd.ctrl != nil {
		if wd.ctrl.RunningState != nil {
			resp.VPNRunning = wd.ctrl.RunningState.IsRunning()
		}

		ver, err := wd.ctrl.GetInstalledCoreVersion()
		if err == nil {
			resp.CoreVersion = ver
		}

		var proxies []api.ProxyInfo
		var activeName string
		if wd.ctrl.APIService != nil {
			proxies = wd.ctrl.APIService.GetProxiesList()
			activeName = wd.ctrl.GetActiveProxyName()
		}
		resp.ProxyCount = len(proxies)
		resp.Selected = activeName

		for _, p := range proxies {
			item := ProxyItem{
				Name:     p.Name,
				Type:     p.ClashType,
				Delay:    p.Delay,
				DelayOK:  p.Delay > 0,
				Selected: p.Name == activeName,
			}
			if p.Delay > 0 {
				item.DelayText = fmt.Sprintf("%dms", p.Delay)
			} else {
				item.DelayText = "⏳"
			}
			resp.Proxies = append(resp.Proxies, item)
		}
	}

	json.NewEncoder(w).Encode(resp)
}

// handleProxies отдаёт только список прокси.
func (wd *WebDashboard) handleProxies(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var items []ProxyItem
	if wd.ctrl != nil && wd.ctrl.APIService != nil {
		proxies := wd.ctrl.APIService.GetProxiesList()
		activeName := wd.ctrl.GetActiveProxyName()

		for _, p := range proxies {
			item := ProxyItem{
				Name:     p.Name,
				Type:     p.ClashType,
				Delay:    p.Delay,
				DelayOK:  p.Delay > 0,
				Selected: p.Name == activeName,
			}
			if p.Delay > 0 {
				item.DelayText = fmt.Sprintf("%dms", p.Delay)
			} else {
				item.DelayText = "⏳"
			}
			items = append(items, item)
		}
	}

	if items == nil {
		items = []ProxyItem{}
	}
	json.NewEncoder(w).Encode(items)
}

// handleToggleVPN переключает VPN вкл/выкл.
func (wd *WebDashboard) handleToggleVPN(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if wd.ctrl == nil {
		http.Error(w, `{"error":"controller not available"}`, http.StatusInternalServerError)
		return
	}

	running := false
	if wd.ctrl.RunningState != nil {
		running = wd.ctrl.RunningState.IsRunning()
	}

	if running {
		go core.StopSingBoxProcess()
	} else {
		go core.StartSingBoxProcess()
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"action":  "toggle_vpn",
		"running": !running,
	})
}

// handleSelectProxy выбирает прокси через Clash API.
func (wd *WebDashboard) handleSelectProxy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}

	if wd.ctrl != nil && wd.ctrl.APIService != nil {
		groupName := wd.ctrl.GetActiveProxyName()
		if groupName == "" {
			groupName = "Proxy"
		}
		baseURL, token, _ := wd.ctrl.APIService.GetClashAPIConfig()
		if baseURL != "" {
			if err := api.SwitchProxy(baseURL, token, groupName, req.Name); err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"ok":    false,
					"error": err.Error(),
				})
				return
			}
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"name": req.Name,
	})
}

// getAPIService — утилита для получения информации о сервисах через контроллер
func getAPIService(ctrl *core.AppController) *services.APIService {
	if ctrl == nil {
		return nil
	}
	return ctrl.APIService
}
