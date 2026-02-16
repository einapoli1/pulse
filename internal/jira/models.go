// Package jira provides a client for the Jira REST API v3.
package jira

import "time"

// Project represents a Jira project.
type Project struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
	Self string `json:"self"`
}

// Board represents a Jira/Agile board.
type Board struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // scrum, kanban
	Self string `json:"self"`
}

// Sprint represents a Jira sprint.
type Sprint struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	State     string `json:"state"` // active, closed, future
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Self      string `json:"self"`
}

// User represents a Jira user (assignee, reporter, etc.).
type User struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress"`
	Active      bool   `json:"active"`
}

// Status represents an issue status.
type Status struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// StatusCategory for grouping statuses.
type StatusCategory struct {
	ID   int    `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// Priority represents an issue priority.
type Priority struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// IssueType represents the type of issue.
type IssueType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Issue represents a Jira issue.
type Issue struct {
	ID     string      `json:"id"`
	Key    string      `json:"key"`
	Self   string      `json:"self"`
	Fields IssueFields `json:"fields"`
}

// IssueFields contains the fields of a Jira issue.
type IssueFields struct {
	Summary     string    `json:"summary"`
	Description *ADF      `json:"description"` // Atlassian Document Format
	Status      Status    `json:"status"`
	Priority    Priority  `json:"priority"`
	Assignee    *User     `json:"assignee"`
	Reporter    *User     `json:"reporter"`
	IssueType   IssueType `json:"issuetype"`
	Created     string    `json:"created"`
	Updated     string    `json:"updated"`
	Labels      []string  `json:"labels"`
}

// ADF is a simplified Atlassian Document Format node.
type ADF struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	Content []ADF  `json:"content,omitempty"`
}

// PlainText extracts plain text from an ADF document.
func (a *ADF) PlainText() string {
	if a == nil {
		return ""
	}
	if a.Text != "" {
		return a.Text
	}
	var out string
	for _, c := range a.Content {
		t := c.PlainText()
		if t != "" {
			if out != "" && c.Type == "paragraph" {
				out += "\n"
			}
			out += t
		}
	}
	return out
}

// Transition represents a Jira issue transition.
type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   Status `json:"to"`
}

// SearchResult is the response from a JQL search.
type SearchResult struct {
	StartAt    int     `json:"startAt"`
	MaxResults int     `json:"maxResults"`
	Total      int     `json:"total"`
	Issues     []Issue `json:"issues"`
}

// ProjectList is the paginated response for projects.
type ProjectList struct {
	Values     []Project `json:"values"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
}

// BoardList is the paginated response for boards.
type BoardList struct {
	Values     []Board `json:"values"`
	MaxResults int     `json:"maxResults"`
	Total      int     `json:"total"`
}

// SprintList is the paginated response for sprints.
type SprintList struct {
	Values     []Sprint `json:"values"`
	MaxResults int      `json:"maxResults"`
}

// TransitionList is the response for issue transitions.
type TransitionList struct {
	Transitions []Transition `json:"transitions"`
}

// Config holds Jira connection settings.
type Config struct {
	URL   string // e.g. https://zonitrnd.atlassian.net
	Email string
	Token string
}

// IsConfigured returns true if all required fields are set.
func (c Config) IsConfigured() bool {
	return c.URL != "" && c.Email != "" && c.Token != ""
}

// Comment represents a Jira comment.
type Comment struct {
	ID      string `json:"id"`
	Author  User   `json:"author"`
	Body    *ADF   `json:"body"`
	Created string `json:"created"`
}

// CheckedAt returns the time the issue was last updated, parsed.
func (i *Issue) UpdatedAt() time.Time {
	t, _ := time.Parse("2006-01-02T15:04:05.000-0700", i.Fields.Updated)
	return t
}
