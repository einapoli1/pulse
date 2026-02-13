package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	configPath := flag.String("config", defaultConfigPath(), "config file path")
	once := flag.Bool("once", false, "check once and exit (no TUI)")
	watch := flag.Bool("watch", false, "check repeatedly without TUI")
	jsonOut := flag.Bool("json", false, "output as JSON (with --once or --watch)")
	initFlag := flag.Bool("init", false, "create sample config file")
	flag.Parse()

	if *initFlag {
		path := *configPath
		if err := writeDefaultConfig(path); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Created sample config at %s\n", path)
		fmt.Println("Edit it with your hosts, then run: pulse")
		return
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		if strings.Contains(err.Error(), "no such file") {
			fmt.Fprintf(os.Stderr, "Run 'pulse --init' to create a sample config.\n")
		}
		os.Exit(1)
	}

	if len(cfg.Hosts) == 0 {
		fmt.Fprintln(os.Stderr, "No hosts configured. Edit your config file.")
		os.Exit(1)
	}

	if *once || *watch {
		for {
			results := checkAllHosts(cfg)
			if *jsonOut {
				printJSON(results)
			} else {
				printTable(results)
			}
			if *once {
				return
			}
			time.Sleep(time.Duration(cfg.Interval) * time.Second)
		}
	}

	// TUI mode
	m := initialModel(cfg, false)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func checkAllHosts(cfg *Config) []HostStatus {
	results := make([]HostStatus, len(cfg.Hosts))
	for i, h := range cfg.Hosts {
		results[i] = checkHost(h)
	}
	return results
}

func printTable(results []HostStatus) {
	for _, r := range results {
		status := "DOWN"
		detail := r.Error
		if r.Online {
			status = "UP"
			parts := []string{}
			if r.CPU != "" {
				parts = append(parts, "load:"+r.CPU)
			}
			if r.Memory != "" {
				parts = append(parts, "mem:"+r.Memory)
			}
			if r.Disk != "" {
				parts = append(parts, "disk:"+r.Disk)
			}
			if r.Uptime != "" {
				parts = append(parts, "up:"+r.Uptime)
			}
			detail = strings.Join(parts, " | ")
		}
		fmt.Printf("%-5s %-20s %s\n", status, r.Config.Label, detail)
	}
}

type jsonResult struct {
	Name    string `json:"name"`
	Host    string `json:"host"`
	Online  bool   `json:"online"`
	CPU     string `json:"cpu,omitempty"`
	Memory  string `json:"memory,omitempty"`
	Disk    string `json:"disk,omitempty"`
	Uptime  string `json:"uptime,omitempty"`
	Error   string `json:"error,omitempty"`
	CheckAt string `json:"checked_at"`
}

func printJSON(results []HostStatus) {
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
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}
