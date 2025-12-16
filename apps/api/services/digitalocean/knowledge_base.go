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
	Name           string                  `json:"name"`
	Description    string                  `json:"description,omitempty"`
	EmbeddingModel string                  `json:"embedding_model_uuid,omitempty"`
	ProjectID      string                  `json:"project_id,omitempty"`
	Region         string                  `json:"region,omitempty"`
	DataSources    []DataSourceCreateInput `json:"datasources,omitempty"`
	// DatabaseID is the UUID of an existing DigitalOcean OpenSearch database.
	// If not provided, a new database is created for each knowledge base.
	// To reuse the same database across multiple knowledge bases, provide the same database_id.
	DatabaseID string `json:"database_id,omitempty"`
}

// DataSourceCreateInput represents a data source input for creating a knowledge base
type DataSourceCreateInput struct {
	BucketName       string                 `json:"bucket_name,omitempty"`
	BucketRegion     string                 `json:"bucket_region,omitempty"`
	SpacesDataSource *SpacesDataSourceInput `json:"spaces_data_source,omitempty"`
}

// SpacesDataSourceInput represents a DigitalOcean Spaces data source
type SpacesDataSourceInput struct {
	BucketName string `json:"bucket_name"`
	Region     string `json:"region"`
	ItemPath   string `json:"item_path,omitempty"`
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
// According to DO API, must contain exactly ONE of: spaces_data_source, web_crawler_data_source, aws_data_source
type CreateDataSourceRequest struct {
	KnowledgeBaseUUID    string                     `json:"knowledge_base_uuid"`
	SpacesDataSource     *SpacesDataSourceInput     `json:"spaces_data_source,omitempty"`
	WebCrawlerDataSource *WebCrawlerDataSourceInput `json:"web_crawler_data_source,omitempty"`
}

// WebCrawlerDataSourceInput represents a web crawler data source configuration
type WebCrawlerDataSourceInput struct {
	BaseURL        string   `json:"base_url"`
	CrawlingOption string   `json:"crawling_option,omitempty"` // SCOPED, FULL
	EmbedMedia     bool     `json:"embed_media,omitempty"`
	ExcludeTags    []string `json:"exclude_tags,omitempty"`
}

// FileUploadRequest represents a request to get presigned URLs for file upload
type FileUploadRequest struct {
	Files []FileUploadInfo `json:"files"`
}

// FileUploadInfo represents info about a file to upload
type FileUploadInfo struct {
	FileName string `json:"file_name"`
	FileSize string `json:"file_size"` // Size in bytes as string
}

// FileUploadResponse represents the response from presigned URL request
type FileUploadResponse struct {
	RequestID string                   `json:"request_id"`
	Uploads   []FilePresignedURLResult `json:"uploads"`
}

// FilePresignedURLResult represents a presigned URL for a single file
type FilePresignedURLResult struct {
	PresignedURL     string    `json:"presigned_url"`
	ObjectKey        string    `json:"object_key"`
	OriginalFileName string    `json:"original_file_name"`
	ExpiresAt        time.Time `json:"expires_at"`
}

// PresignedUploadURL represents a presigned URL for file upload (legacy format)
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

// GetDataSource retrieves a specific data source by listing all and filtering
// Note: The DO API doesn't support GET for a single data source, only LIST
func (c *Client) GetDataSource(ctx context.Context, kbUUID, dsUUID string) (*KnowledgeBaseDataSource, error) {
	// List all data sources and find the one we need
	dataSources, err := c.ListKnowledgeBaseDataSources(ctx, kbUUID)
	if err != nil {
		return nil, err
	}

	for _, ds := range dataSources {
		if ds.UUID == dsUUID {
			dsCopy := ds // Create a copy to return a pointer
			return &dsCopy, nil
		}
	}

	return nil, fmt.Errorf("data source %s not found in knowledge base %s", dsUUID, kbUUID)
}

// CreateDataSource creates a new data source in a knowledge base
func (c *Client) CreateDataSource(ctx context.Context, kbUUID string, req CreateDataSourceRequest) (*DataSource, *PresignedUploadURL, error) {
	endpoint := fmt.Sprintf("/v2/gen-ai/knowledge_bases/%s/data_sources", kbUUID)

	var result struct {
		KnowledgeBaseDataSource DataSource         `json:"knowledge_base_data_source"`
		PresignedURL            PresignedUploadURL `json:"presigned_url,omitempty"`
	}

	if err := c.doRequest(ctx, "POST", endpoint, req, &result); err != nil {
		return nil, nil, err
	}

	return &result.KnowledgeBaseDataSource, &result.PresignedURL, nil
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

// IndexingJob represents an indexing job
type IndexingJob struct {
	UUID              string    `json:"uuid"`
	KnowledgeBaseUUID string    `json:"knowledge_base_uuid"`
	Phase             string    `json:"phase"`  // BATCH_JOB_PHASE_PENDING, BATCH_JOB_PHASE_RUNNING, BATCH_JOB_PHASE_SUCCEEDED
	Status            string    `json:"status"` // INDEX_JOB_STATUS_PENDING, INDEX_JOB_STATUS_IN_PROGRESS, INDEX_JOB_STATUS_COMPLETED
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// DataSourceIndexingJob represents the indexing job status for a specific data source
// This is returned in the last_datasource_indexing_job field from the API
type DataSourceIndexingJob struct {
	DataSourceUUID    string `json:"data_source_uuid"`
	TotalFileCount    string `json:"total_file_count"`
	IndexedFileCount  string `json:"indexed_file_count"`
	TotalBytes        string `json:"total_bytes"`
	TotalBytesIndexed string `json:"total_bytes_indexed"`
	StartedAt         string `json:"started_at"`
	CompletedAt       string `json:"completed_at"`
	IndexedItemCount  string `json:"indexed_item_count"`
	Status            string `json:"status"` // DATA_SOURCE_STATUS_UPDATED, etc.
}

// StartIndexingJobRequest represents a request to start an indexing job
type StartIndexingJobRequest struct {
	KnowledgeBaseUUID string   `json:"knowledge_base_uuid"`
	DataSourceUUIDs   []string `json:"data_source_uuids,omitempty"`
}

// StartIndexingJob starts an indexing job for a knowledge base
func (c *Client) StartIndexingJob(ctx context.Context, req StartIndexingJobRequest) (*IndexingJob, error) {
	endpoint := "/v2/gen-ai/indexing_jobs"

	var result struct {
		Job IndexingJob `json:"job"`
	}

	if err := c.doRequest(ctx, "POST", endpoint, req, &result); err != nil {
		return nil, err
	}

	return &result.Job, nil
}

// GetIndexingJob retrieves an indexing job by UUID
func (c *Client) GetIndexingJob(ctx context.Context, jobUUID string) (*IndexingJob, error) {
	endpoint := fmt.Sprintf("/v2/gen-ai/indexing_jobs/%s", jobUUID)

	var result struct {
		Job IndexingJob `json:"job"`
	}

	if err := c.doRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, err
	}

	return &result.Job, nil
}

// KnowledgeBaseDataSource represents a data source in the knowledge base (full API schema)
type KnowledgeBaseDataSource struct {
	UUID             string    `json:"uuid"`
	BucketName       string    `json:"bucket_name,omitempty"` // Deprecated
	Region           string    `json:"region,omitempty"`      // Deprecated
	ItemPath         string    `json:"item_path,omitempty"`   // Deprecated
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	SpacesDataSource *struct {
		BucketName string `json:"bucket_name"`
		ItemPath   string `json:"item_path"`
		Region     string `json:"region"`
	} `json:"spaces_data_source,omitempty"`
	WebCrawlerDataSource *struct {
		BaseURL string `json:"base_url"`
	} `json:"web_crawler_data_source,omitempty"`
	LastIndexingJob           *IndexingJob           `json:"last_indexing_job,omitempty"`
	LastDataSourceIndexingJob *DataSourceIndexingJob `json:"last_datasource_indexing_job,omitempty"`
}

func (c *Client) ListKnowledgeBaseDataSources(ctx context.Context, kbUUID string) ([]KnowledgeBaseDataSource, error) {
	endpoint := fmt.Sprintf("/v2/gen-ai/knowledge_bases/%s/data_sources", kbUUID)

	var result struct {
		DataSources []KnowledgeBaseDataSource `json:"knowledge_base_data_sources"`
	}

	if err := c.doRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, err
	}

	return result.DataSources, nil
}
