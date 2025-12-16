package digitalocean

import (
	"context"
	"fmt"
	"time"
)

// SpacesAccessKey represents a Spaces access key
type SpacesAccessKey struct {
	AccessKey string    `json:"access_key"` // The Access Key ID (DO API uses "access_key", not "access_key_id")
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// SpacesAccessKeyWithSecret includes the secret (only returned on creation)
type SpacesAccessKeyWithSecret struct {
	SpacesAccessKey
	SecretKey string `json:"secret_key"` // DO API uses "secret_key", not "secret_access_key"
}

// CreateSpacesKeyRequest represents a request to create a Spaces access key
type CreateSpacesKeyRequest struct {
	Name   string           `json:"name"`
	Grants []SpacesKeyGrant `json:"grants"`
}

// SpacesKeyGrant represents a permission grant for a Spaces key
type SpacesKeyGrant struct {
	Bucket     string `json:"bucket"`     // Empty string for fullaccess
	Permission string `json:"permission"` // read, readwrite, fullaccess
}

// ListSpacesKeys lists all Spaces access keys
func (c *Client) ListSpacesKeys(ctx context.Context) ([]SpacesAccessKey, error) {
	endpoint := "/v2/spaces/keys"

	var result struct {
		Keys []SpacesAccessKey `json:"keys"` // DO API uses "keys", not "access_keys"
	}

	if err := c.doRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, err
	}

	return result.Keys, nil
}

// GetSpacesKey retrieves a specific Spaces access key
func (c *Client) GetSpacesKey(ctx context.Context, accessKeyID string) (*SpacesAccessKey, error) {
	endpoint := fmt.Sprintf("/v2/spaces/keys/%s", accessKeyID)

	// Note: DO API returns "key" not "access_key" for get responses
	var result struct {
		Key SpacesAccessKey `json:"key"`
	}

	if err := c.doRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, err
	}

	return &result.Key, nil
}

// CreateSpacesKey creates a new Spaces access key with full access to all buckets
// NOTE: The secret is only returned once upon creation - store it securely!
func (c *Client) CreateSpacesKey(ctx context.Context, name string) (*SpacesAccessKeyWithSecret, error) {
	return c.CreateSpacesKeyWithGrants(ctx, name, nil)
}

// CreateSpacesKeyWithGrants creates a new Spaces access key with specified grants
// If grants is nil or empty, creates a full access key
func (c *Client) CreateSpacesKeyWithGrants(ctx context.Context, name string, grants []SpacesKeyGrant) (*SpacesAccessKeyWithSecret, error) {
	endpoint := "/v2/spaces/keys"

	// Default to fullaccess if no grants specified
	if grants == nil || len(grants) == 0 {
		grants = []SpacesKeyGrant{
			{
				Bucket:     "",           // Empty bucket means all buckets
				Permission: "fullaccess", // Full read/write access
			},
		}
	}

	req := CreateSpacesKeyRequest{
		Name:   name,
		Grants: grants,
	}

	// Note: DO API returns "key" not "access_key" for create responses
	var result struct {
		Key SpacesAccessKeyWithSecret `json:"key"`
	}

	if err := c.doRequest(ctx, "POST", endpoint, req, &result); err != nil {
		return nil, err
	}

	return &result.Key, nil
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
				name, key.AccessKey)
		}
	}

	// Create new key
	newKey, err := c.CreateSpacesKey(ctx, name)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create Spaces key: %w", err)
	}

	return newKey, true, nil
}
