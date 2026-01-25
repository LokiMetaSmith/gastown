package jules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	baseURL = "https://jules.googleapis.com/v1alpha"
)

// Client handles interaction with the Jules API.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Jules API client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewClientFromEnv creates a new client using JULES_API_KEY environment variable.
func NewClientFromEnv() (*Client, error) {
	key := os.Getenv("JULES_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("JULES_API_KEY environment variable not set")
	}
	return NewClient(key), nil
}

// Session represents a Jules session.
type Session struct {
	Name          string         `json:"name"`
	ID            string         `json:"id"`
	Title         string         `json:"title"`
	SourceContext *SourceContext `json:"sourceContext,omitempty"`
	Prompt        string         `json:"prompt,omitempty"`
	Outputs       []SessionOutput `json:"outputs,omitempty"`
}

// SourceContext defines the source for a session.
type SourceContext struct {
	Source            string             `json:"source"`
	GithubRepoContext *GithubRepoContext `json:"githubRepoContext,omitempty"`
}

// GithubRepoContext defines GitHub-specific context.
type GithubRepoContext struct {
	StartingBranch string `json:"startingBranch,omitempty"`
}

// SessionOutput represents outputs from a session.
type SessionOutput struct {
	PullRequest *PullRequestOutput `json:"pullRequest,omitempty"`
}

// PullRequestOutput represents a PR created by the session.
type PullRequestOutput struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// Activity represents a single activity within a session.
type Activity struct {
	Name            string           `json:"name"`
	ID              string           `json:"id"`
	CreateTime      time.Time        `json:"createTime"`
	Originator      string           `json:"originator"` // "agent" or "user"
	PlanGenerated   *PlanGenerated   `json:"planGenerated,omitempty"`
	PlanApproved    *PlanApproved    `json:"planApproved,omitempty"`
	ProgressUpdated *ProgressUpdated `json:"progressUpdated,omitempty"`
	SessionCompleted map[string]interface{} `json:"sessionCompleted,omitempty"`
	Artifacts       []Artifact       `json:"artifacts,omitempty"`
}

type PlanGenerated struct {
	Plan Plan `json:"plan"`
}

type Plan struct {
	ID    string     `json:"id"`
	Steps []PlanStep `json:"steps"`
}

type PlanStep struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Index int    `json:"index"`
}

type PlanApproved struct {
	PlanID string `json:"planId"`
}

type ProgressUpdated struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

type Artifact struct {
	BashOutput *BashOutput `json:"bashOutput,omitempty"`
	ChangeSet  *ChangeSet  `json:"changeSet,omitempty"`
	Media      *Media      `json:"media,omitempty"`
}

type BashOutput struct {
	Command  string `json:"command,omitempty"`
	Output   string `json:"output,omitempty"`
	ExitCode int    `json:"exitCode"`
}

type ChangeSet struct {
	Source   string    `json:"source"`
	GitPatch *GitPatch `json:"gitPatch,omitempty"`
}

type GitPatch struct {
	BaseCommitID           string `json:"baseCommitId,omitempty"`
	UnidiffPatch           string `json:"unidiffPatch,omitempty"`
	SuggestedCommitMessage string `json:"suggestedCommitMessage,omitempty"`
}

type Media struct {
	Data     string `json:"data"`
	MimeType string `json:"mimeType"`
}

// ListSessionsResponse is the response for listing sessions.
type ListSessionsResponse struct {
	Sessions      []Session `json:"sessions"`
	NextPageToken string    `json:"nextPageToken,omitempty"`
}

// ListActivitiesResponse is the response for listing activities.
type ListActivitiesResponse struct {
	Activities    []Activity `json:"activities"`
	NextPageToken string     `json:"nextPageToken,omitempty"`
}

// CreateSessionRequest is the request to create a session.
type CreateSessionRequest struct {
	Prompt              string         `json:"prompt"`
	SourceContext       *SourceContext `json:"sourceContext"`
	AutomationMode      string         `json:"automationMode,omitempty"`
	Title               string         `json:"title,omitempty"`
	RequirePlanApproval bool           `json:"requirePlanApproval,omitempty"`
}

// SendMessageRequest is the request to send a message to a session.
type SendMessageRequest struct {
	Prompt string `json:"prompt"`
}

// Source represents a Jules source.
type Source struct {
	Name       string      `json:"name"`
	ID         string      `json:"id"`
	GithubRepo *GithubRepo `json:"githubRepo,omitempty"`
}

type GithubRepo struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

type ListSourcesResponse struct {
	Sources       []Source `json:"sources"`
	NextPageToken string   `json:"nextPageToken,omitempty"`
}

// ListSources lists available sources.
func (c *Client) ListSources(pageSize int, pageToken string) (*ListSourcesResponse, error) {
	url := fmt.Sprintf("%s/sources?pageSize=%d", baseURL, pageSize)
	if pageToken != "" {
		url += fmt.Sprintf("&pageToken=%s", pageToken)
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var resp ListSourcesResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateSession creates a new session.
func (c *Client) CreateSession(reqBody *CreateSessionRequest) (*Session, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/sessions", baseURL), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	var resp Session
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetSession retrieves a session.
func (c *Client) GetSession(sessionID string) (*Session, error) {
	// sessionID might be "sessions/..." or just ID. API expects resource name usually in URL path
	// but let's assume caller passes ID. If it needs full resource name, we'd adjust.
	// Based on docs: GET https://jules.googleapis.com/v1alpha/sessions/SESSION_ID
	url := fmt.Sprintf("%s/sessions/%s", baseURL, sessionID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var resp Session
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListSessions lists sessions.
func (c *Client) ListSessions(pageSize int, pageToken string) (*ListSessionsResponse, error) {
	url := fmt.Sprintf("%s/sessions?pageSize=%d", baseURL, pageSize)
	if pageToken != "" {
		url += fmt.Sprintf("&pageToken=%s", pageToken)
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var resp ListSessionsResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListActivities lists activities for a session.
func (c *Client) ListActivities(sessionID string, pageSize int, pageToken string) (*ListActivitiesResponse, error) {
	url := fmt.Sprintf("%s/sessions/%s/activities?pageSize=%d", baseURL, sessionID, pageSize)
	if pageToken != "" {
		url += fmt.Sprintf("&pageToken=%s", pageToken)
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var resp ListActivitiesResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SendMessage sends a message to the agent in a session.
func (c *Client) SendMessage(sessionID, prompt string) error {
	reqBody := SendMessageRequest{Prompt: prompt}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/sessions/%s:sendMessage", baseURL, sessionID)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// Response is empty according to docs
	return c.do(req, nil)
}

// ApprovePlan approves a plan in a session.
func (c *Client) ApprovePlan(sessionID string) error {
	url := fmt.Sprintf("%s/sessions/%s:approvePlan", baseURL, sessionID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	return c.do(req, nil)
}

func (c *Client) do(req *http.Request, v interface{}) error {
	req.Header.Set("X-Goog-Api-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}
	return nil
}
