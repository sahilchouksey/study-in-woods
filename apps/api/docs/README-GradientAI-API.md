# DigitalOcean GradientAI Platform API

This is a filtered OpenAPI specification containing only the **GradientAI Platform** endpoints from the complete DigitalOcean API v2.

## 📁 Files

- `digitalocean-GradientAI-Platform-api.yaml` - The filtered OpenAPI 3.0 specification
- `GradientAI-API-Summary.md` - Detailed summary of extracted endpoints
- `README-GradientAI-API.md` - This file

## 📊 Statistics

- **55 unique API endpoints**
- **84 API operations** (GET, POST, PUT, DELETE)
- **187 component schemas**
- **1 tag category**: GradientAI Platform

## 🔑 What is GradientAI Platform?

The DigitalOcean GradientAI Platform API lets you build GPU-powered AI agents with:
- Pre-built or custom foundation models
- Function and agent routes
- RAG pipelines with knowledge bases

## 🎯 Main Features

### Agent Management
- Create, list, update, and delete AI agents
- Manage agent versions with rollback capability
- Control deployment visibility
- Track usage metrics
- Establish parent-child agent relationships

### Knowledge Bases
- Create and manage knowledge bases for RAG
- Upload data sources via presigned URLs
- Trigger and monitor indexing jobs
- Schedule automatic re-indexing

### Evaluation & Testing
- Create evaluation datasets and test cases
- Run evaluation tests
- Retrieve evaluation metrics and results

### External Model Integration
- Integrate Anthropic API keys
- Integrate OpenAI API keys
- OAuth2 support for external services

### Organization
- Organize resources in workspaces
- Move agents between workspaces
- List agents and tests by workspace

## 🚀 Getting Started

### Prerequisites
- A DigitalOcean account
- An API token with appropriate `genai:*` scopes

### Authentication
All endpoints require Bearer token authentication:

```bash
curl -H "Authorization: Bearer $DIGITALOCEAN_TOKEN" \
     https://api.digitalocean.com/v2/gen-ai/agents
```

### Required Scopes
- `genai:read` - Read access to GradientAI resources
- `genai:create` - Create new resources
- `genai:update` - Update existing resources  
- `genai:delete` - Delete resources

## 📚 API Categories

| Category | Endpoints | Description |
|----------|-----------|-------------|
| Agents | 10 | Create and manage AI agents |
| Agent API Keys | 3 | Manage authentication keys for agents |
| Agent Functions | 2 | Attach custom functions to agents |
| Knowledge Bases | 7 | RAG data management |
| Indexing Jobs | 6 | Monitor knowledge base indexing |
| Scheduled Indexing | 3 | Automate knowledge base updates |
| Evaluation | 11 | Test and evaluate agent performance |
| Models | 4 | List and manage foundation models |
| Anthropic Integration | 2 | Integrate Anthropic Claude models |
| OpenAI Integration | 2 | Integrate OpenAI models |
| OAuth2 | 2 | External service authentication |
| Workspaces | 2 | Organize resources |
| Regions | 1 | List available regions |

## 📝 Example Usage

### List All Agents
```bash
curl -X GET \
  -H "Authorization: Bearer $DIGITALOCEAN_TOKEN" \
  https://api.digitalocean.com/v2/gen-ai/agents
```

### Create an Agent
```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $DIGITALOCEAN_TOKEN" \
  -d '{
    "name": "my-agent",
    "model_uuid": "model-uuid-here",
    "instruction": "You are a helpful assistant",
    "region": "nyc3"
  }' \
  https://api.digitalocean.com/v2/gen-ai/agents
```

### Create a Knowledge Base
```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $DIGITALOCEAN_TOKEN" \
  -d '{
    "name": "my-knowledge-base",
    "description": "Product documentation"
  }' \
  https://api.digitalocean.com/v2/gen-ai/knowledge_bases
```

## 🛠️ Using the OpenAPI Spec

### Generate Client SDKs
```bash
# Using OpenAPI Generator
openapi-generator-cli generate \
  -i digitalocean-GradientAI-Platform-api.yaml \
  -g python \
  -o ./python-client

# Other supported languages: java, javascript, go, ruby, php, etc.
```

### Import into Postman
1. Open Postman
2. Click Import
3. Select the `digitalocean-GradientAI-Platform-api.yaml` file
4. Configure your environment with `DIGITALOCEAN_TOKEN`

### API Documentation
```bash
# Generate HTML documentation with Redoc
npx @redocly/cli build-docs digitalocean-GradientAI-Platform-api.yaml

# Or use Swagger UI
docker run -p 8080:8080 \
  -e SWAGGER_JSON=/app/api.yaml \
  -v $(pwd):/app \
  swaggerapi/swagger-ui
```

## 🔗 Resources

- [DigitalOcean API Documentation](https://docs.digitalocean.com/reference/api/)
- [GradientAI Platform Docs](https://docs.digitalocean.com/products/ai/)
- [OpenAPI Specification](https://swagger.io/specification/)

## ⚙️ Extraction Details

- **Source**: DigitalOcean-public.v2.yaml (71,623 lines)
- **Method**: Python script with recursive reference resolution
- **Validation**: YAML structure validated
- **Date**: November 2, 2025

## 📄 License

This API specification is provided by DigitalOcean under the Apache 2.0 license.

## 🤝 Support

For API support, contact:
- **Email**: api-engineering@digitalocean.com
- **Documentation**: https://docs.digitalocean.com/reference/api/

---

Generated from DigitalOcean public API specification v2.0
