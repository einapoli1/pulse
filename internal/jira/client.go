package jira

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a Jira REST API v3 client.
type Client struct {
	config     Config
	httpClient *http.Client
	authHeader string
}

// NewClient creates a new Jira client from config.
func NewClient(cfg Config) *Client {
	auth := base64.StdEncoding.EncodeToString([]byte(cfg.Email + ":" + cfg.Token))
	return &Client{
		config:     cfg,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		authHeader: "Basic " + auth,
	}
}

func (c *Client) do(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	u := strings.TrimRight(c.config.URL, "/") + path
	req, err := http.NewRequest(method, u, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateStr(string(data), 200))
	}

	return data, nil
}

// ListProjects returns all accessible projects.
func (c *Client) ListProjects() ([]Project, error) {
	data, err := c.do("GET", "/rest/api/3/project/search?maxResults=50", nil)
	if err != nil {
		return nil, err
	}
	var result ProjectList
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse projects: %w", err)
	}
	return result.Values, nil
}

// ListBoards returns boards, optionally filtered by project key.
func (c *Client) ListBoards(projectKey string) ([]Board, error) {
	path := "/rest/agile/1.0/board?maxResults=50"
	if projectKey != "" {
		path += "&projectKeyOrId=" + url.QueryEscape(projectKey)
	}
	data, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result BoardList
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse boards: %w", err)
	}
	return result.Values, nil
}

// ListSprints returns sprints for a board.
func (c *Client) ListSprints(boardID int) ([]Sprint, error) {
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/sprint?maxResults=50", boardID)
	data, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result SprintList
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse sprints: %w", err)
	}
	return result.Values, nil
}

// SearchIssues performs a JQL search.
func (c *Client) SearchIssues(jql string, maxResults int) ([]Issue, error) {
	if maxResults <= 0 {
		maxResults = 50
	}
	body := map[string]interface{}{
		"jql":        jql,
		"maxResults": maxResults,
		"fields":     []string{"summary", "status", "priority", "assignee", "reporter", "issuetype", "created", "updated", "labels", "description"},
	}
	data, err := c.do("POST", "/rest/api/3/search", body)
	if err != nil {
		return nil, err
	}
	var result SearchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse search: %w", err)
	}
	return result.Issues, nil
}

// GetIssue fetches a single issue by key (e.g., "PROJ-123").
func (c *Client) GetIssue(key string) (*Issue, error) {
	data, err := c.do("GET", "/rest/api/3/issue/"+url.PathEscape(key), nil)
	if err != nil {
		return nil, err
	}
	var issue Issue
	if err := json.Unmarshal(data, &issue); err != nil {
		return nil, fmt.Errorf("parse issue: %w", err)
	}
	return &issue, nil
}

// AssignIssue assigns an issue to a user by account ID.
func (c *Client) AssignIssue(issueKey, accountID string) error {
	body := map[string]string{"accountId": accountID}
	_, err := c.do("PUT", "/rest/api/3/issue/"+url.PathEscape(issueKey)+"/assignee", body)
	return err
}

// GetTransitions returns available transitions for an issue.
func (c *Client) GetTransitions(issueKey string) ([]Transition, error) {
	data, err := c.do("GET", "/rest/api/3/issue/"+url.PathEscape(issueKey)+"/transitions", nil)
	if err != nil {
		return nil, err
	}
	var result TransitionList
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse transitions: %w", err)
	}
	return result.Transitions, nil
}

// TransitionIssue moves an issue to a new status via transition ID.
func (c *Client) TransitionIssue(issueKey, transitionID string) error {
	body := map[string]interface{}{
		"transition": map[string]string{"id": transitionID},
	}
	_, err := c.do("POST", "/rest/api/3/issue/"+url.PathEscape(issueKey)+"/transitions", body)
	return err
}

// AddComment adds a comment to an issue.
func (c *Client) AddComment(issueKey, text string) error {
	body := map[string]interface{}{
		"body": map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []map[string]interface{}{
				{
					"type": "paragraph",
					"content": []map[string]interface{}{
						{"type": "text", "text": text},
					},
				},
			},
		},
	}
	_, err := c.do("POST", "/rest/api/3/issue/"+url.PathEscape(issueKey)+"/comment", body)
	return err
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
