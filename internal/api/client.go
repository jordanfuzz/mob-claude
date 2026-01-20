package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client handles communication with the mob-claude dashboard API
type Client struct {
	baseURL    string
	httpClient *http.Client
	teamName   string
}

// NewClient creates a new API client
func NewClient(baseURL, teamName string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		teamName: teamName,
	}
}

// Workstream represents a mob programming workstream
type Workstream struct {
	ID        string    `json:"id"`
	TeamID    string    `json:"teamId"`
	RepoURL   string    `json:"repoUrl"`
	Branch    string    `json:"branch"`
	PlanText  string    `json:"planText,omitempty"`
	IsActive  bool      `json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Rotation represents a single driver rotation
type Rotation struct {
	ID           string          `json:"id"`
	WorkstreamID string          `json:"workstreamId"`
	DriverName   string          `json:"driverName"`
	DriverNote   string          `json:"driverNote,omitempty"`
	SummaryTLDR  string          `json:"summaryTldr,omitempty"`
	SummaryJSON  json.RawMessage `json:"summaryJson,omitempty"`
	PlanSnapshot string          `json:"planSnapshot,omitempty"`
	StartedAt    time.Time       `json:"startedAt"`
	EndedAt      time.Time       `json:"endedAt,omitempty"`
}

// Team represents a team in the system
type Team struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	DisplayName string       `json:"displayName,omitempty"`
	Workstreams []Workstream `json:"workstreams,omitempty"`
}

// CreateWorkstreamRequest is the payload for creating a workstream
type CreateWorkstreamRequest struct {
	RepoURL string `json:"repoUrl"`
	Branch  string `json:"branch"`
}

// CreateRotationRequest is the payload for recording a rotation
type CreateRotationRequest struct {
	DriverName   string          `json:"driverName"`
	DriverNote   string          `json:"driverNote,omitempty"`
	SummaryTLDR  string          `json:"summaryTldr,omitempty"`
	SummaryJSON  json.RawMessage `json:"summaryJson,omitempty"`
	PlanSnapshot string          `json:"planSnapshot,omitempty"`
	StartedAt    time.Time       `json:"startedAt"`
}

// UpdatePlanRequest is the payload for updating a workstream's plan
type UpdatePlanRequest struct {
	PlanText string `json:"planText"`
}

// GetTeam fetches the team and its workstreams
func (c *Client) GetTeam() (*Team, error) {
	endpoint := fmt.Sprintf("%s/api/teams/%s", c.baseURL, url.PathEscape(c.teamName))
	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch team: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var team Team
	if err := json.NewDecoder(resp.Body).Decode(&team); err != nil {
		return nil, fmt.Errorf("failed to decode team: %w", err)
	}

	return &team, nil
}

// CreateWorkstream creates or gets a workstream for the given branch
func (c *Client) CreateWorkstream(repoURL, branch string) (*Workstream, error) {
	endpoint := fmt.Sprintf("%s/api/teams/%s/workstreams", c.baseURL, url.PathEscape(c.teamName))

	payload := CreateWorkstreamRequest{
		RepoURL: repoURL,
		Branch:  branch,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create workstream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var workstream Workstream
	if err := json.NewDecoder(resp.Body).Decode(&workstream); err != nil {
		return nil, fmt.Errorf("failed to decode workstream: %w", err)
	}

	return &workstream, nil
}

// GetWorkstream fetches a specific workstream by branch
func (c *Client) GetWorkstream(branch string) (*Workstream, error) {
	endpoint := fmt.Sprintf("%s/api/teams/%s/workstreams/%s",
		c.baseURL, url.PathEscape(c.teamName), url.PathEscape(branch))

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch workstream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var workstream Workstream
	if err := json.NewDecoder(resp.Body).Decode(&workstream); err != nil {
		return nil, fmt.Errorf("failed to decode workstream: %w", err)
	}

	return &workstream, nil
}

// GetPlan fetches the current plan for a workstream
func (c *Client) GetPlan(branch string) (string, error) {
	endpoint := fmt.Sprintf("%s/api/teams/%s/workstreams/%s/plan",
		c.baseURL, url.PathEscape(c.teamName), url.PathEscape(branch))

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to fetch plan: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read plan: %w", err)
	}

	// Response might be JSON with planText field
	var planResp struct {
		PlanText string `json:"planText"`
	}
	if err := json.Unmarshal(body, &planResp); err == nil && planResp.PlanText != "" {
		return planResp.PlanText, nil
	}

	return string(body), nil
}

// UpdatePlan updates the plan for a workstream
func (c *Client) UpdatePlan(branch, planText string) error {
	endpoint := fmt.Sprintf("%s/api/teams/%s/workstreams/%s/plan",
		c.baseURL, url.PathEscape(c.teamName), url.PathEscape(branch))

	payload := UpdatePlanRequest{PlanText: planText}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update plan: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// CreateRotation records a new rotation for a workstream
func (c *Client) CreateRotation(branch string, rotation *CreateRotationRequest) (*Rotation, error) {
	endpoint := fmt.Sprintf("%s/api/teams/%s/workstreams/%s/rotations",
		c.baseURL, url.PathEscape(c.teamName), url.PathEscape(branch))

	body, err := json.Marshal(rotation)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create rotation: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result Rotation
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode rotation: %w", err)
	}

	return &result, nil
}

// Ping checks if the API is reachable
func (c *Client) Ping() error {
	endpoint := fmt.Sprintf("%s/api/health", c.baseURL)
	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return fmt.Errorf("API unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API unhealthy (status %d)", resp.StatusCode)
	}

	return nil
}
