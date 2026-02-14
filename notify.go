package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// StateTracker tracks host online/offline transitions and fires notifications.
type StateTracker struct {
	prev   map[string]bool // host -> was online
	config NotifyConfig
}

func NewStateTracker(cfg NotifyConfig) *StateTracker {
	return &StateTracker{
		prev:   make(map[string]bool),
		config: cfg,
	}
}

// Update checks for state changes and fires notifications. Returns list of transitions.
func (st *StateTracker) Update(results []HostStatus) []string {
	var transitions []string
	for _, r := range results {
		key := r.Config.Host
		wasOnline, seen := st.prev[key]
		st.prev[key] = r.Online

		if !seen {
			continue // first check, no transition
		}
		if wasOnline && !r.Online {
			msg := fmt.Sprintf("%s (%s) went DOWN", r.Config.Label, r.Config.Host)
			transitions = append(transitions, msg)
			go st.notify(r.Config, "down")
		} else if !wasOnline && r.Online {
			msg := fmt.Sprintf("%s (%s) came UP", r.Config.Label, r.Config.Host)
			transitions = append(transitions, msg)
			go st.notify(r.Config, "up")
		}
	}
	return transitions
}

func (st *StateTracker) notify(hc HostConfig, state string) {
	if st.config.Webhook != "" {
		st.webhookNotify(hc, state)
	}
	if st.config.Command != "" {
		st.commandNotify(hc, state)
	}
}

func (st *StateTracker) webhookNotify(hc HostConfig, state string) {
	payload := map[string]string{
		"host":  hc.Host,
		"label": hc.Label,
		"state": state,
		"time":  time.Now().Format(time.RFC3339),
	}
	body, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 10 * time.Second}
	client.Post(st.config.Webhook, "application/json", bytes.NewReader(body)) //nolint:errcheck
}

func (st *StateTracker) commandNotify(hc HostConfig, state string) {
	cmd := st.config.Command
	cmd = strings.ReplaceAll(cmd, "{host}", hc.Host)
	cmd = strings.ReplaceAll(cmd, "{label}", hc.Label)
	cmd = strings.ReplaceAll(cmd, "{state}", state)
	exec.Command("sh", "-c", cmd).Run() //nolint:errcheck
}
