# GradientAI Platform API - Response Structures Documentation

## Overview
This document details all response structures found in the extracted GradientAI Platform API specification.

---

## ✅ Confirmation: All Response Structures Are Present

**YES**, the extracted API file (`digitalocean-GradientAI-Platform-api.yaml`) contains **ALL** the response structures you described, including:

- ✓ Rate limit headers (200, 401, 404, 429, 500)
- ✓ Success response schemas (agents, links, meta)
- ✓ Error response schemas (id, message, request_id)
- ✓ Pagination structures

---

## 1. Success Response (200)

### Response Headers

All successful responses include these rate limit headers:

| Header | Type | Example | Description |
|--------|------|---------|-------------|
| **ratelimit-limit** | integer | `5000` | The default limit on number of requests that can be made per hour (5000) and per minute (250) |
| **ratelimit-remaining** | integer | `4816` | The number of requests in your hourly quota that remain before you hit your request limit |
| **ratelimit-reset** | integer | `1444931833` | Unix epoch time when the oldest request will expire |

### Response Body Schema

**Content-Type:** `application/json`

#### Schema: `apiListAgentsOutputPublic`

```json
{
  "agents": [
    {
      // Array of apiAgentPublic objects
    }
  ],
  "links": {
    "pages": {
      // Pagination links
    }
  },
  "meta": {
    "page": 1,
    "pages": 10,
    "total": 100
  }
}
```

#### Properties

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| **agents** | Array<`apiAgentPublic`> | Yes | List of agent objects |
| **links** | `apiLinks` | Yes | Pagination navigation links |
| **meta** | `apiMeta` | Yes | Metadata about the result set |

---

## 2. Error Responses (401, 404, 429, 500)

### Response Headers

Error responses also include rate limit headers (same as success responses):
- `ratelimit-limit`
- `ratelimit-remaining`
- `ratelimit-reset`

### Response Body Schema

**Content-Type:** `application/json`

#### Schema: `error`

```json
{
  "id": "not_found",
  "message": "The resource you were accessing could not be found.",
  "request_id": "4d9d8375-3c56-4925-a3e7-eb137fed17e9"
}
```

#### Properties

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| **id** | string | **Yes** | A short identifier corresponding to the HTTP status code (e.g., "not_found", "unauthorized") |
| **message** | string | **Yes** | A message providing additional information about the error, including details to help resolve it when possible |
| **request_id** | string | No | Optionally included request ID for reporting bugs or opening support tickets |

### Error Response Types

| Status Code | Response Name | Description |
|-------------|---------------|-------------|
| **401** | `unauthorized` | Authentication failed due to invalid credentials |
| **404** | `not_found` | The resource was not found |
| **429** | `too_many_requests` | The API rate limit has been exceeded |
| **500** | `server_error` | There was a server error |
| **default** | `unexpected_error` | There was an unexpected error |

---

## 3. Nested Response Schemas

### apiAgentPublic (Agent Object)

Represents a single GenAI Agent's configuration.

**Type:** `object`

**Total Properties:** 28

#### Key Properties

| Property | Type | Description |
|----------|------|-------------|
| `uuid` | string | Unique identifier for the agent |
| `name` | string | Agent name |
| `description` | string | Description of agent |
| `instruction` | string | Instructions for the agent |
| `model` | `apiModel` | Model configuration |
| `created_at` | string | Creation date/time |
| `updated_at` | string | Last update date/time |
| `region` | string | Region code where agent is deployed |
| `project_id` | string | DigitalOcean project ID |
| `deployment` | `apiDeployment` | Deployment configuration |
| `tags` | array[string] | Tags to organize your agent |
| `temperature` | number | Model temperature setting |
| `max_tokens` | integer | Maximum tokens for responses |
| `top_p` | number | Top-p sampling parameter |
| `k` | integer | Number of results from knowledge base |
| `provide_citations` | boolean | Whether to provide in-response citations |
| `retrieval_method` | `apiRetrievalMethod` | Knowledge base retrieval method |
| `chatbot` | `apiChatbot` | Chatbot configuration |
| `chatbot_identifiers` | array | Chatbot identifiers |
| `route_uuid` | string | Route UUID |
| `route_name` | string | Route name |
| `route_created_at` | string | Route creation date/time |
| `route_created_by` | string | User ID who created the route |
| `version_hash` | string | Version hash for the agent |
| `url` | string | Agent URL |
| `user_id` | string | User ID of agent owner |
| `template` | string | Template configuration |
| `if_case` | string | Instructions on how to use the route |

---

### apiLinks (Pagination Links)

**Type:** `object`

**Description:** Links to other pages

```json
{
  "pages": {
    "first": "https://api.digitalocean.com/v2/gen-ai/agents?page=1",
    "prev": "https://api.digitalocean.com/v2/gen-ai/agents?page=1",
    "next": "https://api.digitalocean.com/v2/gen-ai/agents?page=3",
    "last": "https://api.digitalocean.com/v2/gen-ai/agents?page=10"
  }
}
```

#### Properties

| Property | Type | Description |
|----------|------|-------------|
| **pages** | `apiPages` | Object containing pagination URLs |

---

### apiMeta (Pagination Metadata)

**Type:** `object`

**Description:** Meta information about the data set

```json
{
  "page": 2,
  "pages": 10,
  "total": 100
}
```

#### Properties

| Property | Type | Description | Example |
|----------|------|-------------|---------|
| **page** | integer (int64) | The current page number | `2` |
| **pages** | integer (int64) | Total number of pages | `10` |
| **total** | integer (int64) | Total amount of items over all pages | `100` |

---

### apiPages (Page Links)

**Type:** `object`

Contains URLs for pagination navigation.

```json
{
  "first": "https://api.digitalocean.com/v2/gen-ai/agents?page=1",
  "prev": "https://api.digitalocean.com/v2/gen-ai/agents?page=1",
  "next": "https://api.digitalocean.com/v2/gen-ai/agents?page=3",
  "last": "https://api.digitalocean.com/v2/gen-ai/agents?page=10"
}
```

#### Properties

| Property | Type | Description |
|----------|------|-------------|
| `first` | string (URL) | Link to first page |
| `prev` | string (URL) | Link to previous page (if applicable) |
| `next` | string (URL) | Link to next page (if applicable) |
| `last` | string (URL) | Link to last page |

---

## 4. Example Complete Response

### Success Response (200) - List Agents

```http
HTTP/1.1 200 OK
Content-Type: application/json
ratelimit-limit: 5000
ratelimit-remaining: 4816
ratelimit-reset: 1444931833

{
  "agents": [
    {
      "uuid": "c441bf77-81d6-11ef-bf8f-4e013e2ddde4",
      "name": "Weather Assistant",
      "description": "An AI agent that provides weather information",
      "instruction": "You are a helpful weather assistant",
      "model": {
        "uuid": "95ea6652-75ed-11ef-bf8f-4e013e2ddde4",
        "name": "gpt-4",
        "provider": "openai"
      },
      "region": "nyc3",
      "project_id": "37455431-84bd-4fa2-94cf-e8486f8f8c5e",
      "created_at": "2024-10-01T12:00:00Z",
      "updated_at": "2024-10-15T14:30:00Z",
      "temperature": 0.7,
      "max_tokens": 1000,
      "tags": ["production", "weather"]
    }
  ],
  "links": {
    "pages": {
      "first": "https://api.digitalocean.com/v2/gen-ai/agents?page=1",
      "next": "https://api.digitalocean.com/v2/gen-ai/agents?page=2",
      "last": "https://api.digitalocean.com/v2/gen-ai/agents?page=5"
    }
  },
  "meta": {
    "page": 1,
    "pages": 5,
    "total": 42
  }
}
```

### Error Response (401) - Unauthorized

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json
ratelimit-limit: 5000
ratelimit-remaining: 4999
ratelimit-reset: 1444931833

{
  "id": "unauthorized",
  "message": "Unable to authenticate you.",
  "request_id": "4d9d8375-3c56-4925-a3e7-eb137fed17e9"
}
```

### Error Response (404) - Not Found

```http
HTTP/1.1 404 Not Found
Content-Type: application/json
ratelimit-limit: 5000
ratelimit-remaining: 4998
ratelimit-reset: 1444931833

{
  "id": "not_found",
  "message": "The resource you were accessing could not be found.",
  "request_id": "7b3a9f12-4e56-4d78-9abc-123456789def"
}
```

### Error Response (429) - Rate Limit Exceeded

```http
HTTP/1.1 429 Too Many Requests
Content-Type: application/json
ratelimit-limit: 5000
ratelimit-remaining: 0
ratelimit-reset: 1444931900
retry-after: 67

{
  "id": "too_many_requests",
  "message": "API Rate limit exceeded.",
  "request_id": "9c8e7d6f-5a4b-3c2d-1e0f-987654321abc"
}
```

---

## 5. Schema Verification Checklist

✅ **All Required Schemas Present:**

- [x] `apiListAgentsOutputPublic` - Success response schema
- [x] `apiAgentPublic` - Individual agent object
- [x] `apiLinks` - Pagination links
- [x] `apiMeta` - Pagination metadata
- [x] `apiPages` - Page link objects
- [x] `error` - Error response schema

✅ **All Response Headers Present:**

- [x] `ratelimit-limit` - Request limit per hour/minute
- [x] `ratelimit-remaining` - Remaining requests
- [x] `ratelimit-reset` - Reset time (Unix epoch)

✅ **All Error Responses Present:**

- [x] `unauthorized` (401)
- [x] `not_found` (404)
- [x] `too_many_requests` (429)
- [x] `server_error` (500)
- [x] `unexpected_error` (default)

---

## Summary

**YES, all response structures you described are present in the extracted API file!**

The file contains complete and accurate response schemas for:
1. ✅ Success responses with rate limit headers
2. ✅ Paginated results with links and metadata
3. ✅ Error responses with id, message, and optional request_id
4. ✅ All HTTP status codes (200, 401, 404, 429, 500)

The extracted `digitalocean-GradientAI-Platform-api.yaml` file is complete and ready to use!

---

**Document Version:** 1.0  
**Last Updated:** November 2, 2025
