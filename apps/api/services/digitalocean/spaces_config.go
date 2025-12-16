package digitalocean

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

var (
	globalSpacesConfig     *SpacesConfig
	globalSpacesConfigOnce sync.Once
	globalSpacesConfigErr  error
)

// GetGlobalSpacesConfig returns the Spaces configuration, auto-generating keys if needed
// This is safe to call multiple times - it will only initialize once
func GetGlobalSpacesConfig() (*SpacesConfig, error) {
	globalSpacesConfigOnce.Do(func() {
		globalSpacesConfig, globalSpacesConfigErr = initGlobalSpacesConfig()
	})
	return globalSpacesConfig, globalSpacesConfigErr
}

// initGlobalSpacesConfig initializes the Spaces configuration
func initGlobalSpacesConfig() (*SpacesConfig, error) {
	config := &SpacesConfig{
		AccessKey: os.Getenv("DO_SPACES_ACCESS_KEY"),
		SecretKey: os.Getenv("DO_SPACES_SECRET_KEY"),
		Bucket:    os.Getenv("DO_SPACES_BUCKET"),
		Region:    os.Getenv("DO_SPACES_REGION"),
		Endpoint:  os.Getenv("DO_SPACES_ENDPOINT"),
		CDNURL:    os.Getenv("DO_SPACES_CDN_ENDPOINT"),
	}

	// Check if bucket and region are configured (required)
	if config.Bucket == "" || config.Region == "" {
		return nil, fmt.Errorf("DO_SPACES_BUCKET and DO_SPACES_REGION must be configured")
	}

	// Set default endpoint if not provided (without https:// prefix for URL construction)
	if config.Endpoint == "" {
		config.Endpoint = fmt.Sprintf("%s.digitaloceanspaces.com", config.Region)
	}

	// If access keys are already configured, use them
	if config.AccessKey != "" && config.SecretKey != "" {
		config.Initialized = true
		log.Println("Spaces: Using configured access keys")
		return config, nil
	}

	// Try to auto-generate keys using DIGITALOCEAN_TOKEN
	doToken := os.Getenv("DIGITALOCEAN_TOKEN")
	if doToken == "" {
		return nil, fmt.Errorf("neither DO_SPACES_ACCESS_KEY/DO_SPACES_SECRET_KEY nor DIGITALOCEAN_TOKEN is configured")
	}

	log.Println("Spaces: No access keys configured, attempting to auto-generate...")

	// Create DO client
	client := NewClient(Config{
		APIToken: doToken,
		Timeout:  30 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Generate a unique key name for this app instance
	keyName := fmt.Sprintf("study-in-woods-auto-%s", config.Bucket)

	// Try to create a new key
	newKey, created, err := client.GetOrCreateSpacesKey(ctx, keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create Spaces key: %w", err)
	}

	if created {
		log.Printf("Spaces: Created new access key '%s' (ID: %s)", keyName, newKey.AccessKey)
		log.Println("Spaces: IMPORTANT - New credentials created. Check your environment or secrets manager.")
		log.Println("Spaces: The secret key has been configured automatically.")

		config.AccessKey = newKey.AccessKey
		config.SecretKey = newKey.SecretKey
		config.Initialized = true
		return config, nil
	}

	// This shouldn't happen since GetOrCreateSpacesKey returns an error if key exists
	return nil, fmt.Errorf("key exists but secret is not available")
}

// IsConfigured returns true if Spaces is properly configured
func (c *SpacesConfig) IsConfigured() bool {
	return c != nil && c.Initialized && c.AccessKey != "" && c.SecretKey != ""
}

// NewSpacesClientFromGlobalConfig creates a SpacesClient from the global config
func NewSpacesClientFromGlobalConfig() (*SpacesClient, error) {
	config, err := GetGlobalSpacesConfig()
	if err != nil {
		return nil, err
	}

	if !config.IsConfigured() {
		return nil, fmt.Errorf("Spaces is not properly configured")
	}

	return NewSpacesClient(*config)
}
