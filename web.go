package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// HostHistory tracks recent check results for sparkline display.
type HostHistory struct {
	Checks []bool    // true=up, false=down (newest last)
	Times  []time.Time
	Max    int
}

func NewHostHistory(max int) *HostHistory {
	return &HostHistory{Max: max}
}

func (h *HostHistory) Add(online bool) {
	h.Checks = append(h.Checks, online)
	h.Times = append(h.Times, time.Now())
	if len(h.Checks) > h.Max {
		h.Checks = h.Checks[1:]
		h.Times = h.Times[1:]
	}
}

// UptimePercent returns the percentage of checks that were online.
func (h *HostHistory) UptimePercent() float64 {
	if len(h.Checks) == 0 {
		return 0
	}
	up := 0
	for _, c := range h.Checks {
		if c {
			up++
		}
	}
	return float64(up) / float64(len(h.Checks)) * 100
}

// Sparkline returns a string of block chars representing uptime history.
func (h *HostHistory) Sparkline() string {
	var sb []byte
	for _, c := range h.Checks {
		if c {
			sb = append(sb, 0xE2, 0x96, 0x88) // █ (full block)
		} else {
			sb = append(sb, 0xE2, 0x96, 0x91) // ░ (light shade)
		}
	}
	return string(sb)
}

// WebServer serves a simple dashboard for Pulse host monitoring.
type WebServer struct {
	cfg     *Config
	tracker *StateTracker
	mu      sync.RWMutex
	latest  []HostStatus
	history map[string]*HostHistory // keyed by host name
	port    int
}

func NewWebServer(cfg *Config, port int) *WebServer {
	history := make(map[string]*HostHistory)
	for _, h := range cfg.Hosts {
		history[h.Name] = NewHostHistory(60)
	}
	return &WebServer{
		cfg:     cfg,
		tracker: NewStateTracker(cfg.Notify),
		history: history,
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
		for _, r := range results {
			if h, ok := ws.history[r.Config.Name]; ok {
				h.Add(r.Online)
			}
		}
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
		jr := jsonResult{
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
		if h, ok := ws.history[r.Config.Name]; ok && len(h.Checks) > 0 {
			jr.Sparkline = h.Sparkline()
			jr.UptimePercent = h.UptimePercent()
			jr.CheckCount = len(h.Checks)
		}
		out[i] = jr
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
  .sparkline { font-family: monospace; font-size: 0.6rem; letter-spacing: -1px; margin-top: 0.5rem; color: #22c55e; line-height: 1; }
  .sparkline .down-char { color: #ef4444; }
  .uptime-pct { font-size: 0.75rem; color: #888; margin-top: 0.25rem; }
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
      const sparkline = h.sparkline ? ` + "`" + `<div class="sparkline">${h.sparkline.split('').map(c => c === '█' ? c : ` + "`" + `<span class="down-char">${c}</span>` + "`" + `).join('')}</div><div class="uptime-pct">${h.uptime_percent.toFixed(1)}% uptime (${h.check_count} checks)</div>` + "`" + ` : '';
      return ` + "`" + `<div class="card ${cls}">
        <div class="card-header">
          <span class="host-name">${h.name}</span>
          <span class="badge ${cls}">${cls}</span>
        </div>
        <div class="host-addr">${h.host}</div>
        ${metrics}
        ${sparkline}
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
