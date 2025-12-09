package digitalocean

import (
	"context"
	"fmt"
	"time"
)

// SpacesAccessKey represents a Spaces access key
type SpacesAccessKey struct {
	AccessKeyID string    `json:"access_key_id"`
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"created_at"`
}

// SpacesAccessKeyWithSecret includes the secret (only returned on creation)
type SpacesAccessKeyWithSecret struct {
	SpacesAccessKey
	SecretAccessKey string `json:"secret_access_key"`
}

// CreateSpacesKeyRequest represents a request to create a Spaces access key
type CreateSpacesKeyRequest struct {
	Name string `json:"name"`
}

// ListSpacesKeys lists all Spaces access keys
func (c *Client) ListSpacesKeys(ctx context.Context) ([]SpacesAccessKey, error) {
	endpoint := "/v2/spaces/keys"

	var result struct {
		AccessKeys []SpacesAccessKey `json:"access_keys"`
	}

	if err := c.doRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, err
	}

	return result.AccessKeys, nil
}

// GetSpacesKey retrieves a specific Spaces access key
func (c *Client) GetSpacesKey(ctx context.Context, accessKeyID string) (*SpacesAccessKey, error) {
	endpoint := fmt.Sprintf("/v2/spaces/keys/%s", accessKeyID)

	var result struct {
		AccessKey SpacesAccessKey `json:"access_key"`
	}

	if err := c.doRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, err
	}

	return &result.AccessKey, nil
}

// CreateSpacesKey creates a new Spaces access key
// NOTE: The secret is only returned once upon creation - store it securely!
func (c *Client) CreateSpacesKey(ctx context.Context, name string) (*SpacesAccessKeyWithSecret, error) {
	endpoint := "/v2/spaces/keys"

	req := CreateSpacesKeyRequest{
		Name: name,
	}

	var result struct {
		AccessKey SpacesAccessKeyWithSecret `json:"access_key"`
	}

	if err := c.doRequest(ctx, "POST", endpoint, req, &result); err != nil {
		return nil, err
	}

	return &result.AccessKey, nil
}

// DeleteSpacesKey deletes a Spaces access key
func (c *Client) DeleteSpacesKey(ctx context.Context, accessKeyID string) error {
	endpoint := fmt.Sprintf("/v2/spaces/keys/%s", accessKeyID)
	return c.doRequest(ctx, "DELETE", endpoint, nil, nil)
}

// GetOrCreateSpacesKey gets an existing key by name or creates a new one
// Returns the key and whether it was newly created
func (c *Client) GetOrCreateSpacesKey(ctx context.Context, name string) (*SpacesAccessKeyWithSecret, bool, error) {
	// First, list existing keys to find one with matching name
	keys, err := c.ListSpacesKeys(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("failed to list Spaces keys: %w", err)
	}

	// Check if key with this name already exists
	for _, key := range keys {
		if key.Name == name {
			// Key exists but we don't have the secret
			// Return nil to indicate the key exists but secret is unavailable
			return nil, false, fmt.Errorf("Spaces key '%s' already exists (ID: %s) but secret is not retrievable. "+
				"Either delete the existing key and let the app create a new one, "+
				"or manually set DO_SPACES_ACCESS_KEY and DO_SPACES_SECRET_KEY in your .env file",
				name, key.AccessKeyID)
		}
	}

	// Create new key
	newKey, err := c.CreateSpacesKey(ctx, name)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create Spaces key: %w", err)
	}

	return newKey, true, nil
}
