package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// WebServer serves a simple dashboard for Pulse host monitoring.
type WebServer struct {
	cfg     *Config
	tracker *StateTracker
	mu      sync.RWMutex
	latest  []HostStatus
	port    int
}

func NewWebServer(cfg *Config, port int) *WebServer {
	return &WebServer{
		cfg:     cfg,
		tracker: NewStateTracker(cfg.Notify),
		port:    port,
	}
}

func (ws *WebServer) Run() error {
	// Start background checker
	go ws.pollLoop()

	mux := http.NewServeMux()
	mux.HandleFunc("/", ws.handleDashboard)
	mux.HandleFunc("/api/status", ws.handleAPI)

	addr := fmt.Sprintf(":%d", ws.port)
	fmt.Printf("Pulse web dashboard: http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func (ws *WebServer) pollLoop() {
	for {
		results := checkAllHosts(ws.cfg)
		ws.mu.Lock()
		ws.latest = results
		ws.tracker.Update(results)
		ws.mu.Unlock()
		time.Sleep(time.Duration(ws.cfg.Interval) * time.Second)
	}
}

func (ws *WebServer) handleAPI(w http.ResponseWriter, r *http.Request) {
	ws.mu.RLock()
	results := ws.latest
	ws.mu.RUnlock()

	out := make([]jsonResult, len(results))
	for i, r := range results {
		out[i] = jsonResult{
			Name:    r.Config.Label,
			Host:    r.Config.Host,
			Online:  r.Online,
			CPU:     r.CPU,
			Memory:  r.Memory,
			Disk:    r.Disk,
			Uptime:  r.Uptime,
			Error:   r.Error,
			CheckAt: r.LastCheck.Format(time.RFC3339),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (ws *WebServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, dashboardHTML)
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Pulse</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif; background: #0f1117; color: #e0e0e0; padding: 2rem; }
  h1 { font-size: 1.5rem; margin-bottom: 1.5rem; color: #fff; }
  h1 span { color: #6366f1; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 1rem; }
  .card { background: #1a1d27; border-radius: 12px; padding: 1.25rem; border: 1px solid #2a2d3a; transition: border-color 0.2s; }
  .card.up { border-left: 4px solid #22c55e; }
  .card.down { border-left: 4px solid #ef4444; }
  .card-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.75rem; }
  .host-name { font-weight: 600; font-size: 1.1rem; }
  .badge { padding: 0.2rem 0.6rem; border-radius: 9999px; font-size: 0.75rem; font-weight: 600; text-transform: uppercase; }
  .badge.up { background: #22c55e20; color: #22c55e; }
  .badge.down { background: #ef444420; color: #ef4444; }
  .host-addr { color: #888; font-size: 0.85rem; margin-bottom: 0.75rem; }
  .metrics { display: grid; grid-template-columns: 1fr 1fr; gap: 0.5rem; }
  .metric { background: #12141c; border-radius: 8px; padding: 0.5rem 0.75rem; }
  .metric-label { font-size: 0.7rem; color: #666; text-transform: uppercase; letter-spacing: 0.05em; }
  .metric-value { font-size: 0.95rem; font-weight: 500; margin-top: 0.15rem; }
  .error-msg { color: #ef4444; font-size: 0.85rem; margin-top: 0.5rem; }
  .footer { margin-top: 2rem; color: #555; font-size: 0.8rem; text-align: center; }
  .loading { text-align: center; padding: 3rem; color: #666; }
</style>
</head>
<body>
<h1><span>&#9679;</span> Pulse</h1>
<div id="grid" class="grid"><div class="loading">Loading...</div></div>
<div class="footer">Auto-refreshes every <span id="interval">30</span>s</div>
<script>
async function refresh() {
  try {
    const res = await fetch('/api/status');
    const hosts = await res.json();
    const grid = document.getElementById('grid');
    if (!hosts || hosts.length === 0) {
      grid.innerHTML = '<div class="loading">No hosts configured</div>';
      return;
    }
    grid.innerHTML = hosts.map(h => {
      const cls = h.online ? 'up' : 'down';
      const metrics = h.online ? ` + "`" + `
        <div class="metrics">
          ${h.cpu ? ` + "`" + `<div class="metric"><div class="metric-label">Load</div><div class="metric-value">${h.cpu}</div></div>` + "`" + ` : ''}
          ${h.memory ? ` + "`" + `<div class="metric"><div class="metric-label">Memory</div><div class="metric-value">${h.memory}</div></div>` + "`" + ` : ''}
          ${h.disk ? ` + "`" + `<div class="metric"><div class="metric-label">Disk</div><div class="metric-value">${h.disk}</div></div>` + "`" + ` : ''}
          ${h.uptime ? ` + "`" + `<div class="metric"><div class="metric-label">Uptime</div><div class="metric-value">${h.uptime}</div></div>` + "`" + ` : ''}
        </div>` + "`" + ` : ` + "`" + `<div class="error-msg">${h.error || 'Unreachable'}</div>` + "`" + `;
      return ` + "`" + `<div class="card ${cls}">
        <div class="card-header">
          <span class="host-name">${h.name}</span>
          <span class="badge ${cls}">${cls}</span>
        </div>
        <div class="host-addr">${h.host}</div>
        ${metrics}
      </div>` + "`" + `;
    }).join('');
  } catch (e) {
    console.error('Refresh failed:', e);
  }
}
refresh();
setInterval(refresh, 30000);
</script>
</body>
</html>`
