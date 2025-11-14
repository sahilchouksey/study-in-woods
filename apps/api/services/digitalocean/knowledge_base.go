package digitalocean

import (
	"context"
	"fmt"
	"time"
)

// KnowledgeBase represents a knowledge base for RAG
type KnowledgeBase struct {
	UUID           string    `json:"uuid"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	EmbeddingModel string    `json:"embedding_model"`
	Status         string    `json:"status"` // active, indexing, failed
	DataSources    []string  `json:"data_sources,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CreateKnowledgeBaseRequest represents a request to create a knowledge base
type CreateKnowledgeBaseRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	EmbeddingModel string `json:"embedding_model,omitempty"`
}

// UpdateKnowledgeBaseRequest represents a request to update a knowledge base
type UpdateKnowledgeBaseRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// DataSource represents a data source in a knowledge base
type DataSource struct {
	UUID              string    `json:"uuid"`
	KnowledgeBaseUUID string    `json:"knowledge_base_uuid"`
	Name              string    `json:"name"`
	Type              string    `json:"type"`   // file, url
	Status            string    `json:"status"` // pending, processing, indexed, failed
	FileURL           string    `json:"file_url,omitempty"`
	FileName          string    `json:"file_name,omitempty"`
	FileSize          int64     `json:"file_size,omitempty"`
	ChunkCount        int       `json:"chunk_count,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// CreateDataSourceRequest represents a request to create a data source
type CreateDataSourceRequest struct {
	Name string `json:"name"`
	Type string `json:"type"` // "file"
}

// PresignedUploadURL represents a presigned URL for file upload
type PresignedUploadURL struct {
	URL       string            `json:"url"`
	Fields    map[string]string `json:"fields"`
	ExpiresAt time.Time         `json:"expires_at"`
}

// ListKnowledgeBases retrieves all knowledge bases
func (c *Client) ListKnowledgeBases(ctx context.Context, opts *ListOptions) ([]KnowledgeBase, *Pagination, error) {
	endpoint := "/v2/gen-ai/knowledge_bases"
	if opts != nil && opts.Page > 0 {
		endpoint = fmt.Sprintf("%s?page=%d&per_page=%d", endpoint, opts.Page, opts.PerPage)
	}

	var result struct {
		KnowledgeBases []KnowledgeBase `json:"knowledge_bases"`
		Links          Links           `json:"links"`
		Meta           struct {
			Total int `json:"total"`
		} `json:"meta"`
	}

	if err := c.doRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, nil, err
	}

	pagination := &Pagination{
		Total: result.Meta.Total,
		Count: len(result.KnowledgeBases),
		Links: result.Links,
	}
	if opts != nil {
		pagination.CurrentPage = opts.Page
		pagination.PerPage = opts.PerPage
	}

	return result.KnowledgeBases, pagination, nil
}

// GetKnowledgeBase retrieves a specific knowledge base by UUID
func (c *Client) GetKnowledgeBase(ctx context.Context, uuid string) (*KnowledgeBase, error) {
	endpoint := fmt.Sprintf("/v2/gen-ai/knowledge_bases/%s", uuid)

	var result struct {
		KnowledgeBase KnowledgeBase `json:"knowledge_base"`
	}

	if err := c.doRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, err
	}

	return &result.KnowledgeBase, nil
}

// CreateKnowledgeBase creates a new knowledge base
func (c *Client) CreateKnowledgeBase(ctx context.Context, req CreateKnowledgeBaseRequest) (*KnowledgeBase, error) {
	endpoint := "/v2/gen-ai/knowledge_bases"

	var result struct {
		KnowledgeBase KnowledgeBase `json:"knowledge_base"`
	}

	if err := c.doRequest(ctx, "POST", endpoint, req, &result); err != nil {
		return nil, err
	}

	return &result.KnowledgeBase, nil
}

// UpdateKnowledgeBase updates an existing knowledge base
func (c *Client) UpdateKnowledgeBase(ctx context.Context, uuid string, req UpdateKnowledgeBaseRequest) (*KnowledgeBase, error) {
	endpoint := fmt.Sprintf("/v2/gen-ai/knowledge_bases/%s", uuid)

	var result struct {
		KnowledgeBase KnowledgeBase `json:"knowledge_base"`
	}

	if err := c.doRequest(ctx, "PUT", endpoint, req, &result); err != nil {
		return nil, err
	}

	return &result.KnowledgeBase, nil
}

// DeleteKnowledgeBase deletes a knowledge base
func (c *Client) DeleteKnowledgeBase(ctx context.Context, uuid string) error {
	endpoint := fmt.Sprintf("/v2/gen-ai/knowledge_bases/%s", uuid)
	return c.doRequest(ctx, "DELETE", endpoint, nil, nil)
}

// ListDataSources retrieves all data sources for a knowledge base
func (c *Client) ListDataSources(ctx context.Context, kbUUID string, opts *ListOptions) ([]DataSource, *Pagination, error) {
	endpoint := fmt.Sprintf("/v2/gen-ai/knowledge_bases/%s/data_sources", kbUUID)
	if opts != nil && opts.Page > 0 {
		endpoint = fmt.Sprintf("%s?page=%d&per_page=%d", endpoint, opts.Page, opts.PerPage)
	}

	var result struct {
		DataSources []DataSource `json:"data_sources"`
		Links       Links        `json:"links"`
		Meta        struct {
			Total int `json:"total"`
		} `json:"meta"`
	}

	if err := c.doRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, nil, err
	}

	pagination := &Pagination{
		Total: result.Meta.Total,
		Count: len(result.DataSources),
		Links: result.Links,
	}
	if opts != nil {
		pagination.CurrentPage = opts.Page
		pagination.PerPage = opts.PerPage
	}

	return result.DataSources, pagination, nil
}

// GetDataSource retrieves a specific data source
func (c *Client) GetDataSource(ctx context.Context, kbUUID, dsUUID string) (*DataSource, error) {
	endpoint := fmt.Sprintf("/v2/gen-ai/knowledge_bases/%s/data_sources/%s", kbUUID, dsUUID)

	var result struct {
		DataSource DataSource `json:"data_source"`
	}

	if err := c.doRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, err
	}

	return &result.DataSource, nil
}

// CreateDataSource creates a new data source in a knowledge base
func (c *Client) CreateDataSource(ctx context.Context, kbUUID string, req CreateDataSourceRequest) (*DataSource, *PresignedUploadURL, error) {
	endpoint := fmt.Sprintf("/v2/gen-ai/knowledge_bases/%s/data_sources", kbUUID)

	var result struct {
		DataSource   DataSource         `json:"data_source"`
		PresignedURL PresignedUploadURL `json:"presigned_url,omitempty"`
	}

	if err := c.doRequest(ctx, "POST", endpoint, req, &result); err != nil {
		return nil, nil, err
	}

	return &result.DataSource, &result.PresignedURL, nil
}

// DeleteDataSource deletes a data source from a knowledge base
func (c *Client) DeleteDataSource(ctx context.Context, kbUUID, dsUUID string) error {
	endpoint := fmt.Sprintf("/v2/gen-ai/knowledge_bases/%s/data_sources/%s", kbUUID, dsUUID)
	return c.doRequest(ctx, "DELETE", endpoint, nil, nil)
}

// TriggerIndexing triggers re-indexing of a knowledge base
func (c *Client) TriggerIndexing(ctx context.Context, kbUUID string) error {
	endpoint := fmt.Sprintf("/v2/gen-ai/knowledge_bases/%s/index", kbUUID)
	return c.doRequest(ctx, "POST", endpoint, nil, nil)
}
