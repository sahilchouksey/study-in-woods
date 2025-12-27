package digitalocean

import (
	"context"
	"fmt"
	"time"
)

// Agent represents a DigitalOcean AI agent
type Agent struct {
	UUID             string               `json:"uuid"`
	Name             string               `json:"name"`
	Description      string               `json:"description"`
	ModelID          string               `json:"model_id"`
	Instructions     string               `json:"instructions"`
	Temperature      float64              `json:"temperature"`
	TopP             float64              `json:"top_p"`
	Status           string               `json:"status"` // active, inactive
	ProvideCitations bool                 `json:"provide_citations,omitempty"`
	RetrievalMethod  string               `json:"retrieval_method,omitempty"`
	K                int                  `json:"k,omitempty"` // How many results from KB
	KnowledgeBases   []AgentKnowledgeBase `json:"knowledge_bases,omitempty"`
	Deployment       *AgentDeployment     `json:"deployment,omitempty"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

// AgentKnowledgeBase represents a knowledge base attached to an agent
type AgentKnowledgeBase struct {
	UUID            string       `json:"uuid"`
	Name            string       `json:"name"`
	Region          string       `json:"region"`
	DatabaseID      string       `json:"database_id"`
	LastIndexingJob *IndexingJob `json:"last_indexing_job,omitempty"`
	AddedToAgentAt  time.Time    `json:"added_to_agent_at"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

// AgentDeployment represents deployment info for an agent
type AgentDeployment struct {
	UUID       string    `json:"uuid"`
	URL        string    `json:"url"`
	Status     string    `json:"status"`     // STATUS_RUNNING, STATUS_PENDING
	Visibility string    `json:"visibility"` // VISIBILITY_PLAYGROUND, VISIBILITY_PUBLIC
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// AgentAPIKey represents an API key for an agent
type AgentAPIKey struct {
	UUID      string    `json:"uuid"`
	Name      string    `json:"name"`
	SecretKey string    `json:"secret_key,omitempty"` // Only returned on creation
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
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
	UUID             string   `json:"uuid,omitempty"`
	Name             string   `json:"name,omitempty"`
	Description      string   `json:"description,omitempty"`
	Instructions     string   `json:"instruction,omitempty"` // Note: API uses "instruction" not "instructions"
	Temperature      *float64 `json:"temperature,omitempty"` // Pointer to distinguish 0 from unset
	TopP             *float64 `json:"top_p,omitempty"`
	KnowledgeBases   []string `json:"knowledge_bases,omitempty"`
	ProvideCitations *bool    `json:"provide_citations,omitempty"` // Pointer to distinguish false from unset
	RetrievalMethod  string   `json:"retrieval_method,omitempty"`
	K                *int     `json:"k,omitempty"` // How many results from KB
	MaxTokens        *int     `json:"max_tokens,omitempty"`
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
// Uses GenAI rate limiting since this is a resource-intensive operation
func (c *Client) CreateAgent(ctx context.Context, req CreateAgentRequest) (*Agent, error) {
	endpoint := "/v2/gen-ai/agents"

	var result struct {
		Agent Agent `json:"agent"`
	}

	// Use GenAI rate limiting for agent creation (more conservative)
	if err := c.doRequestGenAI(ctx, "POST", endpoint, req, &result); err != nil {
		return nil, err
	}

	return &result.Agent, nil
}

// UpdateAgent updates an existing agent
func (c *Client) UpdateAgent(ctx context.Context, uuid string, req UpdateAgentRequest) (*Agent, error) {
	endpoint := fmt.Sprintf("/v2/gen-ai/agents/%s", uuid)

	// Ensure UUID is set in request
	req.UUID = uuid

	var result struct {
		Agent Agent `json:"agent"`
	}

	if err := c.doRequest(ctx, "PUT", endpoint, req, &result); err != nil {
		return nil, err
	}

	return &result.Agent, nil
}

// EnableAgentCitations enables citation support for an agent
// When enabled, agent responses will include source references from the knowledge base
func (c *Client) EnableAgentCitations(ctx context.Context, agentUUID string) (*Agent, error) {
	citations := true
	return c.UpdateAgent(ctx, agentUUID, UpdateAgentRequest{
		ProvideCitations: &citations,
	})
}

// DisableAgentCitations disables citation support for an agent
func (c *Client) DisableAgentCitations(ctx context.Context, agentUUID string) (*Agent, error) {
	citations := false
	return c.UpdateAgent(ctx, agentUUID, UpdateAgentRequest{
		ProvideCitations: &citations,
	})
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

// CreateAgentAPIKeyRequest represents a request to create an agent API key
type CreateAgentAPIKeyRequest struct {
	Name string `json:"name"`
}

// CreateAgentAPIKey creates an API key for an agent
// The secret key is only returned on creation and cannot be retrieved later
// Uses GenAI rate limiting since this is a resource-intensive operation
func (c *Client) CreateAgentAPIKey(ctx context.Context, agentUUID, name string) (*AgentAPIKey, error) {
	endpoint := fmt.Sprintf("/v2/gen-ai/agents/%s/api_keys", agentUUID)

	req := CreateAgentAPIKeyRequest{
		Name: name,
	}

	// DO API returns api_key_info, not api_key
	var result struct {
		APIKeyInfo AgentAPIKey `json:"api_key_info"`
	}

	// Use GenAI rate limiting for API key creation (more conservative)
	if err := c.doRequestGenAI(ctx, "POST", endpoint, req, &result); err != nil {
		return nil, err
	}

	return &result.APIKeyInfo, nil
}

// ListAgentAPIKeys lists all API keys for an agent
func (c *Client) ListAgentAPIKeys(ctx context.Context, agentUUID string) ([]AgentAPIKey, error) {
	endpoint := fmt.Sprintf("/v2/gen-ai/agents/%s/api_keys", agentUUID)

	var result struct {
		APIKeys []AgentAPIKey `json:"api_keys"`
	}

	if err := c.doRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, err
	}

	return result.APIKeys, nil
}

// DeleteAgentAPIKey deletes an agent API key
func (c *Client) DeleteAgentAPIKey(ctx context.Context, agentUUID, keyUUID string) error {
	endpoint := fmt.Sprintf("/v2/gen-ai/agents/%s/api_keys/%s", agentUUID, keyUUID)
	return c.doRequest(ctx, "DELETE", endpoint, nil, nil)
}

// GetAgentDeploymentURL returns the deployment URL for an agent
// Returns empty string if agent is not deployed
func (c *Client) GetAgentDeploymentURL(ctx context.Context, agentUUID string) (string, error) {
	agent, err := c.GetAgent(ctx, agentUUID)
	if err != nil {
		return "", err
	}

	if agent.Deployment == nil || agent.Deployment.URL == "" {
		return "", fmt.Errorf("agent %s has no deployment URL", agentUUID)
	}

	return agent.Deployment.URL, nil
}

// DeploymentVisibility represents agent deployment visibility options
type DeploymentVisibility string

const (
	// VisibilityUnknown - The status of the deployment is unknown
	VisibilityUnknown DeploymentVisibility = "VISIBILITY_UNKNOWN"
	// VisibilityDisabled - The deployment is disabled and will no longer service requests
	VisibilityDisabled DeploymentVisibility = "VISIBILITY_DISABLED"
	// VisibilityPublic - The deployment is public and will service requests from the public internet
	VisibilityPublic DeploymentVisibility = "VISIBILITY_PUBLIC"
	// VisibilityPrivate - The deployment is private and will only service requests from other agents, or through API keys
	VisibilityPrivate DeploymentVisibility = "VISIBILITY_PRIVATE"
)

// UpdateAgentDeploymentVisibilityRequest represents a request to update agent deployment visibility
type UpdateAgentDeploymentVisibilityRequest struct {
	UUID       string               `json:"uuid"`
	Visibility DeploymentVisibility `json:"visibility"`
}

// UpdateAgentDeploymentVisibilityResponse represents the response from updating deployment visibility
type UpdateAgentDeploymentVisibilityResponse struct {
	Agent Agent `json:"agent"`
}

// DeployAgent deploys an agent by setting its visibility to public or private
// This triggers DO to provision the agent and generate a deployment URL
// Uses GenAI rate limiting since this is a resource-intensive operation
func (c *Client) DeployAgent(ctx context.Context, agentUUID string, visibility DeploymentVisibility) (*Agent, error) {
	endpoint := fmt.Sprintf("/v2/gen-ai/agents/%s/deployment_visibility", agentUUID)

	req := UpdateAgentDeploymentVisibilityRequest{
		UUID:       agentUUID,
		Visibility: visibility,
	}

	var result UpdateAgentDeploymentVisibilityResponse
	// Use GenAI rate limiting for agent deployment (more conservative)
	if err := c.doRequestGenAI(ctx, "PUT", endpoint, req, &result); err != nil {
		return nil, err
	}

	return &result.Agent, nil
}

// IsAgentDeployed checks if an agent has a usable deployment with a URL
// An agent is considered deployed if it has a URL, even if status is still deploying
func (c *Client) IsAgentDeployed(ctx context.Context, agentUUID string) (bool, *Agent, error) {
	agent, err := c.GetAgent(ctx, agentUUID)
	if err != nil {
		return false, nil, err
	}

	if agent.Deployment == nil {
		return false, agent, nil
	}

	// Check if deployment has a URL (it's usable even while STATUS_DEPLOYING)
	// Or if it's fully running
	isDeployed := agent.Deployment.URL != "" ||
		agent.Deployment.Status == "STATUS_RUNNING"
	return isDeployed, agent, nil
}

// WaitForAgentDeployment waits for an agent to be fully deployed with a URL
func (c *Client) WaitForAgentDeployment(ctx context.Context, agentUUID string, timeout time.Duration) (*Agent, error) {
	deadline := time.Now().Add(timeout)
	pollInterval := 5 * time.Second

	for time.Now().Before(deadline) {
		isDeployed, agent, err := c.IsAgentDeployed(ctx, agentUUID)
		if err != nil {
			return nil, err
		}

		if isDeployed {
			return agent, nil
		}

		// Log current status
		status := "no deployment"
		if agent.Deployment != nil {
			status = agent.Deployment.Status
		}
		fmt.Printf("  Agent deployment status: %s, waiting...\n", status)

		time.Sleep(pollInterval)
	}

	return nil, fmt.Errorf("timeout waiting for agent deployment")
}
