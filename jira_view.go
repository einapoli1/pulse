package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jsnapoli1/pulse/internal/dispatch"
	"github.com/jsnapoli1/pulse/internal/jira"
)

// jiraView levels
type jiraLevel int

const (
	levelProjects jiraLevel = iota
	levelIssues
	levelDetail
)

type jiraModel struct {
	client    *jira.Client
	store     *dispatch.Store
	hosts     []HostConfig // available hosts for dispatch
	configured bool

	level    jiraLevel
	cursor   int
	loading  bool
	err      string

	projects []jira.Project
	selProj  string // selected project key

	issues   []jira.Issue
	jql      string

	selIssue *jira.Issue

	// dispatch prompt
	dispatching   bool
	dispatchCursor int

	width  int
	height int
}

// Messages
type jiraProjectsMsg struct{ projects []jira.Project; err error }
type jiraIssuesMsg struct{ issues []jira.Issue; err error }
type jiraDispatchedMsg struct{ err error }

func newJiraModel(cfg jira.Config, hosts []HostConfig, store *dispatch.Store) jiraModel {
	m := jiraModel{
		store:      store,
		hosts:      hosts,
		configured: cfg.IsConfigured(),
	}
	if m.configured {
		m.client = jira.NewClient(cfg)
	}
	return m
}

func (m jiraModel) loadProjects() tea.Cmd {
	return func() tea.Msg {
		projects, err := m.client.ListProjects()
		return jiraProjectsMsg{projects, err}
	}
}

func (m jiraModel) loadIssues() tea.Cmd {
	return func() tea.Msg {
		jql := fmt.Sprintf("project = %s ORDER BY updated DESC", m.selProj)
		issues, err := m.client.SearchIssues(jql, 50)
		return jiraIssuesMsg{issues, err}
	}
}

func (m jiraModel) Init() tea.Cmd {
	if !m.configured {
		return nil
	}
	m.loading = true
	return m.loadProjects()
}

func (m jiraModel) Update(msg tea.Msg) (jiraModel, tea.Cmd) {
	switch msg := msg.(type) {
	case jiraProjectsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.projects = msg.projects
			m.err = ""
		}
		m.cursor = 0

	case jiraIssuesMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.issues = msg.issues
			m.err = ""
		}
		m.cursor = 0

	case jiraDispatchedMsg:
		m.dispatching = false
		if msg.err != nil {
			m.err = msg.err.Error()
		}

	case tea.KeyMsg:
		if m.dispatching {
			return m.updateDispatch(msg)
		}
		return m.updateNav(msg)
	}
	return m, nil
}

func (m jiraModel) updateNav(msg tea.KeyMsg) (jiraModel, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		max := m.listLen() - 1
		if max < 0 {
			max = 0
		}
		if m.cursor < max {
			m.cursor++
		}
	case "enter", "l":
		return m.enter()
	case "esc", "h", "backspace":
		return m.back()
	case "d":
		// dispatch current issue
		if m.level == levelIssues && len(m.issues) > 0 && len(m.hosts) > 0 {
			m.selIssue = &m.issues[m.cursor]
			m.dispatching = true
			m.dispatchCursor = 0
		} else if m.level == levelDetail && m.selIssue != nil && len(m.hosts) > 0 {
			m.dispatching = true
			m.dispatchCursor = 0
		}
	case "r":
		return m.refresh()
	}
	return m, nil
}

func (m jiraModel) updateDispatch(msg tea.KeyMsg) (jiraModel, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.dispatchCursor > 0 {
			m.dispatchCursor--
		}
	case "down", "j":
		if m.dispatchCursor < len(m.hosts)-1 {
			m.dispatchCursor++
		}
	case "enter":
		if m.selIssue != nil && m.dispatchCursor < len(m.hosts) {
			target := m.hosts[m.dispatchCursor].Label
			m.store.Assign(m.selIssue.Key, m.selIssue.Fields.Summary, target)
			err := m.store.Save()
			m.dispatching = false
			return m, func() tea.Msg { return jiraDispatchedMsg{err} }
		}
	case "esc":
		m.dispatching = false
	}
	return m, nil
}

func (m jiraModel) enter() (jiraModel, tea.Cmd) {
	switch m.level {
	case levelProjects:
		if len(m.projects) > 0 {
			m.selProj = m.projects[m.cursor].Key
			m.level = levelIssues
			m.loading = true
			m.cursor = 0
			return m, m.loadIssues()
		}
	case levelIssues:
		if len(m.issues) > 0 {
			issue := m.issues[m.cursor]
			m.selIssue = &issue
			m.level = levelDetail
			m.cursor = 0
		}
	}
	return m, nil
}

func (m jiraModel) back() (jiraModel, tea.Cmd) {
	switch m.level {
	case levelDetail:
		m.level = levelIssues
		m.selIssue = nil
		m.cursor = 0
	case levelIssues:
		m.level = levelProjects
		m.issues = nil
		m.selProj = ""
		m.cursor = 0
	}
	return m, nil
}

func (m jiraModel) refresh() (jiraModel, tea.Cmd) {
	m.loading = true
	m.err = ""
	switch m.level {
	case levelProjects:
		return m, m.loadProjects()
	case levelIssues:
		return m, m.loadIssues()
	}
	return m, nil
}

func (m jiraModel) listLen() int {
	switch m.level {
	case levelProjects:
		return len(m.projects)
	case levelIssues:
		return len(m.issues)
	}
	return 0
}

func (m jiraModel) View() string {
	if !m.configured {
		return notConfiguredView()
	}

	var b strings.Builder

	// Breadcrumb
	crumb := dimStyle.Render("Jira")
	if m.selProj != "" {
		crumb += dimStyle.Render(" › ") + labelStyle.Render(m.selProj)
	}
	if m.selIssue != nil {
		crumb += dimStyle.Render(" › ") + labelStyle.Render(m.selIssue.Key)
	}
	b.WriteString(crumb + "\n\n")

	if m.loading {
		b.WriteString(dimStyle.Render("Loading...") + "\n")
		return b.String()
	}
	if m.err != "" {
		b.WriteString(offlineStyle.Render("Error: "+m.err) + "\n")
	}

	// Dispatch overlay
	if m.dispatching && m.selIssue != nil {
		return m.dispatchView()
	}

	switch m.level {
	case levelProjects:
		b.WriteString(m.projectsView())
	case levelIssues:
		b.WriteString(m.issuesView())
	case levelDetail:
		b.WriteString(m.detailView())
	}

	b.WriteString("\n" + dimStyle.Render("j/k:nav  enter:select  esc:back  d:dispatch  r:refresh"))
	return b.String()
}

func (m jiraModel) projectsView() string {
	if len(m.projects) == 0 {
		return dimStyle.Render("No projects found.\n")
	}
	var b strings.Builder
	for i, p := range m.projects {
		cursor := "  "
		if i == m.cursor {
			cursor = "▸ "
		}
		line := fmt.Sprintf("%s%s  %s", cursor, labelStyle.Render(p.Key), dimStyle.Render(p.Name))
		b.WriteString(line + "\n")
	}
	return b.String()
}

func (m jiraModel) issuesView() string {
	if len(m.issues) == 0 {
		return dimStyle.Render("No issues found.\n")
	}
	var b strings.Builder
	for i, issue := range m.issues {
		cursor := "  "
		if i == m.cursor {
			cursor = "▸ "
		}
		status := statusStyle(issue.Fields.Status.Name)
		assignee := dimStyle.Render("unassigned")
		if issue.Fields.Assignee != nil {
			assignee = dimStyle.Render(issue.Fields.Assignee.DisplayName)
		}
		line := fmt.Sprintf("%s%s %s %-12s %s",
			cursor,
			labelStyle.Render(issue.Key),
			status,
			assignee,
			issue.Fields.Summary,
		)
		if len(line) > m.width-2 && m.width > 40 {
			line = line[:m.width-5] + "..."
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func (m jiraModel) detailView() string {
	if m.selIssue == nil {
		return ""
	}
	i := m.selIssue
	var b strings.Builder
	b.WriteString(labelStyle.Render(i.Key) + "  " + i.Fields.Summary + "\n\n")
	b.WriteString(fmt.Sprintf("  Status:   %s\n", statusStyle(i.Fields.Status.Name)))
	b.WriteString(fmt.Sprintf("  Type:     %s\n", i.Fields.IssueType.Name))
	b.WriteString(fmt.Sprintf("  Priority: %s\n", i.Fields.Priority.Name))
	assignee := "unassigned"
	if i.Fields.Assignee != nil {
		assignee = i.Fields.Assignee.DisplayName
	}
	b.WriteString(fmt.Sprintf("  Assignee: %s\n", assignee))
	if i.Fields.Reporter != nil {
		b.WriteString(fmt.Sprintf("  Reporter: %s\n", i.Fields.Reporter.DisplayName))
	}
	if len(i.Fields.Labels) > 0 {
		b.WriteString(fmt.Sprintf("  Labels:   %s\n", strings.Join(i.Fields.Labels, ", ")))
	}
	b.WriteString(fmt.Sprintf("  Updated:  %s\n", i.Fields.Updated))

	if i.Fields.Description != nil {
		desc := i.Fields.Description.PlainText()
		if desc != "" {
			b.WriteString("\n  " + dimStyle.Render("Description:") + "\n")
			for _, line := range strings.Split(desc, "\n") {
				b.WriteString("    " + line + "\n")
			}
		}
	}

	// Show dispatch info
	assignments := m.store.ForTarget("")
	for _, a := range m.store.All() {
		if a.IssueKey == i.Key {
			b.WriteString(fmt.Sprintf("\n  📋 Dispatched to: %s (%s)", a.Target, a.Status))
		}
	}
	_ = assignments

	return b.String()
}

func (m jiraModel) dispatchView() string {
	var b strings.Builder
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)

	b.WriteString(labelStyle.Render("Dispatch "+m.selIssue.Key) + "\n")
	b.WriteString(dimStyle.Render(m.selIssue.Fields.Summary) + "\n\n")
	b.WriteString("Select target host:\n\n")

	for i, h := range m.hosts {
		cursor := "  "
		if i == m.dispatchCursor {
			cursor = "▸ "
		}
		b.WriteString(fmt.Sprintf("%s%s (%s)\n", cursor, h.Label, h.Host))
	}
	b.WriteString("\n" + dimStyle.Render("enter:assign  esc:cancel"))

	return border.Render(b.String())
}

func notConfiguredView() string {
	return lipgloss.NewStyle().Padding(1, 2).Render(
		offlineStyle.Render("Jira not configured") + "\n\n" +
			dimStyle.Render("Set environment variables:") + "\n" +
			"  JIRA_URL       " + dimStyle.Render("(e.g. https://zonitrnd.atlassian.net)") + "\n" +
			"  JIRA_EMAIL     " + dimStyle.Render("(your Atlassian email)") + "\n" +
			"  JIRA_API_TOKEN " + dimStyle.Render("(API token from id.atlassian.com)") + "\n\n" +
			dimStyle.Render("Or add to config file under 'jira:' section."),
	)
}

func statusStyle(status string) string {
	s := strings.ToLower(status)
	switch {
	case s == "done" || s == "closed" || s == "resolved":
		return onlineStyle.Render(status)
	case s == "in progress" || s == "in review":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(status)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Render(status)
	}
}
