package digitalocean

import (
	"context"
	"fmt"
	"time"
)

// Agent represents a DigitalOcean AI agent
type Agent struct {
	UUID           string    `json:"uuid"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	ModelID        string    `json:"model_id"`
	Instructions   string    `json:"instructions"`
	Temperature    float64   `json:"temperature"`
	TopP           float64   `json:"top_p"`
	Status         string    `json:"status"` // active, inactive
	KnowledgeBases []string  `json:"knowledge_bases,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CreateAgentRequest represents a request to create an agent
type CreateAgentRequest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	ModelUUID      string   `json:"model_uuid"`
	ProjectID      string   `json:"project_id,omitempty"`
	Region         string   `json:"region,omitempty"`
	Instructions   string   `json:"instruction,omitempty"`
	Temperature    float64  `json:"temperature,omitempty"`
	TopP           float64  `json:"top_p,omitempty"`
	KnowledgeBases []string `json:"knowledge_base_uuid,omitempty"`
}

// UpdateAgentRequest represents a request to update an agent
type UpdateAgentRequest struct {
	Name           string   `json:"name,omitempty"`
	Description    string   `json:"description,omitempty"`
	Instructions   string   `json:"instructions,omitempty"`
	Temperature    float64  `json:"temperature,omitempty"`
	TopP           float64  `json:"top_p,omitempty"`
	KnowledgeBases []string `json:"knowledge_bases,omitempty"`
}

// AgentUsage represents usage statistics for an agent
type AgentUsage struct {
	UUID         string    `json:"uuid"`
	TotalTokens  int       `json:"total_tokens"`
	PromptTokens int       `json:"prompt_tokens"`
	OutputTokens int       `json:"output_tokens"`
	Requests     int       `json:"requests"`
	Period       string    `json:"period"`
	StartDate    time.Time `json:"start_date"`
	EndDate      time.Time `json:"end_date"`
}

// ListAgents retrieves all agents
func (c *Client) ListAgents(ctx context.Context, opts *ListOptions) ([]Agent, *Pagination, error) {
	endpoint := "/v2/gen-ai/agents"
	if opts != nil && opts.Page > 0 {
		endpoint = fmt.Sprintf("%s?page=%d&per_page=%d", endpoint, opts.Page, opts.PerPage)
	}

	var result struct {
		Agents []Agent `json:"agents"`
		Links  Links   `json:"links"`
		Meta   struct {
			Total int `json:"total"`
		} `json:"meta"`
	}

	if err := c.doRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, nil, err
	}

	pagination := &Pagination{
		Total: result.Meta.Total,
		Count: len(result.Agents),
		Links: result.Links,
	}
	if opts != nil {
		pagination.CurrentPage = opts.Page
		pagination.PerPage = opts.PerPage
	}

	return result.Agents, pagination, nil
}

// GetAgent retrieves a specific agent by UUID
func (c *Client) GetAgent(ctx context.Context, uuid string) (*Agent, error) {
	endpoint := fmt.Sprintf("/v2/gen-ai/agents/%s", uuid)

	var result struct {
		Agent Agent `json:"agent"`
	}

	if err := c.doRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, err
	}

	return &result.Agent, nil
}

// CreateAgent creates a new agent
func (c *Client) CreateAgent(ctx context.Context, req CreateAgentRequest) (*Agent, error) {
	endpoint := "/v2/gen-ai/agents"

	var result struct {
		Agent Agent `json:"agent"`
	}

	if err := c.doRequest(ctx, "POST", endpoint, req, &result); err != nil {
		return nil, err
	}

	return &result.Agent, nil
}

// UpdateAgent updates an existing agent
func (c *Client) UpdateAgent(ctx context.Context, uuid string, req UpdateAgentRequest) (*Agent, error) {
	endpoint := fmt.Sprintf("/v2/gen-ai/agents/%s", uuid)

	var result struct {
		Agent Agent `json:"agent"`
	}

	if err := c.doRequest(ctx, "PUT", endpoint, req, &result); err != nil {
		return nil, err
	}

	return &result.Agent, nil
}

// DeleteAgent deletes an agent
func (c *Client) DeleteAgent(ctx context.Context, uuid string) error {
	endpoint := fmt.Sprintf("/v2/gen-ai/agents/%s", uuid)
	return c.doRequest(ctx, "DELETE", endpoint, nil, nil)
}

// GetAgentUsage retrieves usage statistics for an agent
func (c *Client) GetAgentUsage(ctx context.Context, uuid string, startDate, endDate time.Time) (*AgentUsage, error) {
	endpoint := fmt.Sprintf("/v2/gen-ai/agents/%s/usage?start_date=%s&end_date=%s",
		uuid,
		startDate.Format("2006-01-02"),
		endDate.Format("2006-01-02"),
	)

	var result struct {
		Usage AgentUsage `json:"usage"`
	}

	if err := c.doRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, err
	}

	return &result.Usage, nil
}

// AttachKnowledgeBase attaches a knowledge base to an agent
func (c *Client) AttachKnowledgeBase(ctx context.Context, agentUUID, kbUUID string) error {
	// Use the path-based endpoint for single KB attachment
	endpoint := fmt.Sprintf("/v2/gen-ai/agents/%s/knowledge_bases/%s", agentUUID, kbUUID)

	// This endpoint doesn't require a body - just POST to the path
	return c.doRequest(ctx, "POST", endpoint, nil, nil)
}

// DetachKnowledgeBase detaches a knowledge base from an agent
func (c *Client) DetachKnowledgeBase(ctx context.Context, agentUUID, kbUUID string) error {
	endpoint := fmt.Sprintf("/v2/gen-ai/agents/%s/knowledge_bases/%s", agentUUID, kbUUID)
	return c.doRequest(ctx, "DELETE", endpoint, nil, nil)
}
