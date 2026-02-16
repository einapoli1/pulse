// Package dispatch manages assignment of Jira issues to hosts/agents.
package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Status represents the state of a dispatched task.
type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in-progress"
	StatusDone       Status = "done"
	StatusFailed     Status = "failed"
)

// Assignment links a Jira issue to a target host or agent.
type Assignment struct {
	ID        string    `json:"id"`         // unique assignment ID
	IssueKey  string    `json:"issue_key"`  // e.g. "PROJ-123"
	Summary   string    `json:"summary"`    // issue summary for display
	Target    string    `json:"target"`     // hostname, IP, or agent name
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Note      string    `json:"note,omitempty"`
}

// Store persists assignments in a JSON file.
type Store struct {
	path        string
	mu          sync.RWMutex
	Assignments []Assignment `json:"assignments"`
}

// DefaultPath returns ~/.pulse/dispatch.json.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pulse", "dispatch.json")
}

// NewStore loads or creates a dispatch store at the given path.
func NewStore(path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	s := &Store{path: path}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, s); err != nil {
			return nil, fmt.Errorf("parse dispatch file: %w", err)
		}
	}
	return s, nil
}

// Save writes the store to disk.
func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

// Assign creates a new assignment.
func (s *Store) Assign(issueKey, summary, target string) *Assignment {
	s.mu.Lock()
	defer s.mu.Unlock()

	a := Assignment{
		ID:        fmt.Sprintf("%s→%s", issueKey, target),
		IssueKey:  issueKey,
		Summary:   summary,
		Target:    target,
		Status:    StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	// Replace existing assignment for same issue+target
	for i, existing := range s.Assignments {
		if existing.IssueKey == issueKey && existing.Target == target {
			s.Assignments[i] = a
			return &s.Assignments[i]
		}
	}
	s.Assignments = append(s.Assignments, a)
	return &s.Assignments[len(s.Assignments)-1]
}

// UpdateStatus changes the status of an assignment.
func (s *Store) UpdateStatus(id string, status Status, note string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Assignments {
		if s.Assignments[i].ID == id {
			s.Assignments[i].Status = status
			s.Assignments[i].Note = note
			s.Assignments[i].UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("assignment %q not found", id)
}

// ForTarget returns all assignments for a given target.
func (s *Store) ForTarget(target string) []Assignment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Assignment
	for _, a := range s.Assignments {
		if a.Target == target {
			out = append(out, a)
		}
	}
	return out
}

// All returns all assignments.
func (s *Store) All() []Assignment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Assignment, len(s.Assignments))
	copy(out, s.Assignments)
	return out
}

// Remove deletes an assignment by ID.
func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Assignments {
		if s.Assignments[i].ID == id {
			s.Assignments = append(s.Assignments[:i], s.Assignments[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("assignment %q not found", id)
}
