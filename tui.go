package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	onlineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))

	offlineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	labelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	selectedStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	normalStyle = lipgloss.NewStyle().
			Padding(0, 1)
)

type viewTab int

const (
	tabHosts viewTab = iota
	tabJira
)

type model struct {
	config   *Config
	hosts    []HostStatus
	cursor   int
	checking bool
	spinner  spinner.Model
	width    int
	height   int
	once     bool
	tab      viewTab
	jira     jiraModel
}

type checkDoneMsg struct {
	results []HostStatus
}

type tickMsg time.Time

func initialModel(cfg *Config, once bool, jm jiraModel) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("62"))

	hosts := make([]HostStatus, len(cfg.Hosts))
	for i, h := range cfg.Hosts {
		hosts[i] = HostStatus{Config: h}
	}

	return model{
		config:  cfg,
		hosts:   hosts,
		spinner: s,
		once:    once,
		jira:    jm,
	}
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.spinner.Tick,
		m.runChecks(),
	}
	if jiraCmd := m.jira.Init(); jiraCmd != nil {
		cmds = append(cmds, jiraCmd)
	}
	return tea.Batch(cmds...)
}

func (m model) runChecks() tea.Cmd {
	return func() tea.Msg {
		return checkDoneMsg{results: checkAllHosts(m.config)}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			if m.tab == tabHosts {
				m.tab = tabJira
			} else {
				m.tab = tabHosts
			}
			return m, nil
		case "1":
			m.tab = tabHosts
			return m, nil
		case "2":
			m.tab = tabJira
			return m, nil
		}
		if m.tab == tabJira {
			var cmd tea.Cmd
			m.jira, cmd = m.jira.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.hosts)-1 {
				m.cursor++
			}
		case "r":
			m.checking = true
			return m, m.runChecks()
		}

	case checkDoneMsg:
		m.hosts = msg.results
		m.checking = false
		if m.once {
			return m, tea.Quit
		}
		return m, tea.Tick(time.Duration(m.config.Interval)*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})

	case tickMsg:
		m.checking = true
		return m, m.runChecks()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.jira.width = msg.Width
		m.jira.height = msg.Height

	// Forward Jira-specific messages
	case jiraProjectsMsg, jiraIssuesMsg, jiraDispatchedMsg:
		var cmd tea.Cmd
		m.jira, cmd = m.jira.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	title := titleStyle.Render(" ⚡ Pulse ")
	if m.checking && m.tab == tabHosts {
		title += " " + m.spinner.View() + " checking..."
	}

	// Tab bar
	hostsTab := dimStyle.Render(" 1:Hosts ")
	jiraTab := dimStyle.Render(" 2:Jira ")
	if m.tab == tabHosts {
		hostsTab = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("62")).Render(" 1:Hosts ")
	} else {
		jiraTab = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("62")).Render(" 2:Jira ")
	}
	b.WriteString(title + "  " + hostsTab + jiraTab + "\n\n")

	if m.tab == tabJira {
		b.WriteString(m.jira.View())
		return b.String()
	}

	for i, h := range m.hosts {
		style := normalStyle
		if i == m.cursor {
			style = selectedStyle
		}

		status := offlineStyle.Render("● DOWN")
		if h.Online {
			status = onlineStyle.Render("● UP  ")
		} else if h.LastCheck.IsZero() {
			status = dimStyle.Render("● ----")
		}

		label := labelStyle.Render(h.Config.Label)
		host := dimStyle.Render(fmt.Sprintf("(%s@%s)", h.Config.User, h.Config.Host))

		line := fmt.Sprintf("%s %s %s", status, label, host)

		if h.Online {
			details := []string{}
			if h.CPU != "" {
				details = append(details, fmt.Sprintf("load:%s", h.CPU))
			}
			if h.Memory != "" {
				details = append(details, fmt.Sprintf("mem:%s", h.Memory))
			}
			if h.Disk != "" {
				details = append(details, fmt.Sprintf("disk:%s", h.Disk))
			}
			if len(details) > 0 {
				line += "  " + dimStyle.Render(strings.Join(details, " | "))
			}
		} else if h.Error != "" && i == m.cursor {
			line += "\n    " + offlineStyle.Render(truncate(h.Error, 60))
		}

		if !h.LastCheck.IsZero() {
			ago := time.Since(h.LastCheck).Truncate(time.Second)
			line += "  " + dimStyle.Render(fmt.Sprintf("[%s ago]", ago))
		}

		b.WriteString(style.Render(line) + "\n")
	}

	b.WriteString("\n" + dimStyle.Render("j/k:nav  r:refresh  tab/1/2:switch view  q:quit"))

	return b.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
