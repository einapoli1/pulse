# Pulse

A minimal Go terminal dashboard that monitors multiple hosts at a glance.

Shows real-time status of SSH-reachable machines: up/down, CPU, memory, disk, last seen. Built with [bubbletea](https://github.com/charmbracelet/bubbletea) for a clean TUI.

## Why
Jack (and I) manage several machines — Arch PC, Ubuntu VM, Raspberry Pi, MacBook Air. Checking each one individually is tedious. Pulse gives a single-pane view.

## Features
- Configure hosts in `~/.config/pulse/hosts.yaml`
- SSH-based health checks (no agent needed)
- Color-coded status (green/yellow/red)
- Auto-refresh on configurable interval
- Expandable host details on selection

## Usage
```
pulse                    # launch TUI dashboard
pulse --web              # web dashboard on :9100
pulse --web --port 8888  # custom port
pulse --once             # check once and exit
pulse --once --json      # JSON output
pulse --watch            # continuous checks (no TUI)
pulse --config hosts.yaml # custom config
pulse --once             # single check, no TUI (for scripts/cron)
```
