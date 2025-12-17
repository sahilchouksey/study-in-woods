# Study in Woods - Full Stack Monorepo

A comprehensive educational platform with AI-powered chat, document management, and analytics capabilities.

Built with Turborepo for optimized builds and development workflow.

## 🏗️ Monorepo Structure

```
study-in-woods/
├── apps/
│   ├── api/          # Backend API (Go + Fiber + PostgreSQL)
│   └── web/          # Frontend Application (Next.js + React)
├── package.json      # Root package.json with Turborepo
├── turbo.json        # Turborepo configuration
├── pnpm-workspace.yaml
├── .gitignore        # Root gitignore for entire monorepo
└── README.md         # This file
```

---

## 📦 Applications

### Backend API (`apps/api`)
**Tech Stack**: Go, Fiber v2, GORM, PostgreSQL, Redis, DigitalOcean AI Gateway

Complete RESTful API with 96 endpoints covering:
- ✅ Authentication & Authorization (JWT-based)
- ✅ University, Course, Semester, Subject Management
- ✅ Document Management with AI Integration
- ✅ AI-Powered Chat with DigitalOcean
- ✅ Analytics & Monitoring
- ✅ External API Access (Encrypted Keys)
- ✅ Admin Panel (29 endpoints)
- ✅ Background Cron Jobs (6 tasks)
- ✅ Database Seeding

**Lines of Code**: 12,046 lines across 75 Go files

### Frontend Application (`apps/web`)
**Tech Stack**: Next.js 15, React 19, TypeScript, Tailwind CSS, shadcn/ui

Modern web application providing user interface for:
- User authentication and registration
- Course and subject browsing
- Document uploads and management
- AI chat interface
- Analytics dashboard
- Admin panel

📖 **Documentation**: See [`apps/web/README.md`](apps/web/README.md)

---

## 🚀 Quick Start

### Prerequisites
- **Docker** & **Docker Compose** (recommended)
- **Go** 1.21+ (for backend development)
- **Node.js** 18+ (for frontend)
- **npm** 9+ (for package management)
- **DigitalOcean Account** (optional, for AI features and storage)

### Option 1: Docker Setup (Recommended)

#### 1. Clone the Repository

```bash
git clone https://github.com/sahilchouksey/study-in-woods.git
cd study-in-woods
```

#### 2. Start Database Services with Docker

```bash
# Start PostgreSQL and Redis containers
npm run docker:up

# Check if containers are running
npm run docker:ps

# View logs
npm run docker:logs
```

This will start:
- PostgreSQL on `localhost:5432`
- Redis on `localhost:6379`

#### 3. Install Dependencies

```bash
# Install all workspace dependencies (includes Turborepo)
npm install

# Install Go dependencies for backend
cd apps/api && go mod download && cd ../..
```

#### 4. Setup Backend Environment

```bash
cd apps/api

# Copy environment file
cp .env.example .env
# Edit .env if needed (Docker configs are already set correctly)

# Run database migrations
make db-migrate

# Seed database with initial data
make db-seed

cd ../..
```

#### 5. Start Development

```bash
# From the root directory

# Start both apps in parallel
npm run dev

# Or start individual apps:
npm run api:dev    # Backend only
npm run web:dev    # Frontend only
```

Backend will run on `http://localhost:8080`  
Frontend will run on `http://localhost:3000`

**Default Admin Credentials**:
- Email: `admin@studyinwoods.com`
- Password: `Admin123!`

⚠️ **Change password after first login!**

---

### Option 2: Manual Setup (Without Docker)

If you prefer not to use Docker, you need to install and configure PostgreSQL and Redis manually.

#### Prerequisites
- **PostgreSQL** 13+ installed and running
- **Redis** 6+ installed and running

#### Start Services

```bash
# Start PostgreSQL
sudo systemctl start postgresql

# Start Redis
sudo systemctl start redis

# Create database
sudo -u postgres psql -c "CREATE DATABASE study_in_woods;"
```

Then follow steps 1, 3, 4, and 5 from the Docker setup above.

---

### Docker Commands

```bash
# Start containers
npm run docker:up

# Stop containers
npm run docker:down

# View logs
npm run docker:logs

# Check status
npm run docker:ps

# Restart containers
npm run docker:restart

# Stop and remove all data (⚠️ deletes database!)
npm run docker:clean
```

---

## 📋 Environment Variables

### Backend (`apps/api/.env`)

```env
# Database
DATABASE_URL=postgresql://user:pass@host:port/dbname
DB_HOST=localhost
DB_PORT=5432
DB_NAME=study_in_woods
DB_USER_NAME=postgres
DB_PASSWORD=yourpassword
DB_SSL_MODE=disable

# Redis
REDIS_URL=redis://localhost:6379

# Security
JWT_SECRET=<base64-encoded-32-bytes>
ENCRYPTION_KEY=<base64-encoded-32-bytes>

# DigitalOcean
DIGITALOCEAN_TOKEN=dop_v1_xxxxx
DO_SPACES_BUCKET=study-in-woods
DO_SPACES_REGION=blr1
DO_SPACES_ENDPOINT=https://blr1.digitaloceanspaces.com
DO_SPACES_ACCESS_KEY=<spaces-access-key>
DO_SPACES_SECRET_KEY=<spaces-secret-key>

# Application
GO_ENV=development
PORT=8080
CRON_ENABLED=true
```

### Frontend (`apps/web/.env.local`)

```env
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXT_PUBLIC_APP_NAME=Study in Woods
```

---

## 🛠️ Development Commands

### Turborepo Commands (Root)

```bash
# Development
npm run dev           # Start all apps in parallel
npm run api:dev       # Start backend only
npm run web:dev       # Start frontend only

# Build
npm run build         # Build all apps
npm run api:build     # Build backend only
npm run web:build     # Build frontend only

# Start production
npm run start         # Start all apps in production mode

# Linting
npm run lint          # Lint all apps

# Clean
npm run clean         # Clean all build artifacts

# Testing
npm run test          # Run tests for all apps
```

### Backend Commands (`apps/api`)

```bash
cd apps/api

# Development
make dev              # Start with hot reload
make dev-docker       # Start with Docker Compose

# Build
make build            # Build production binary
make build-docker     # Build Docker image

# Database
make db-migrate       # Run migrations
make db-seed          # Seed initial data
make db-reset         # Reset database

# Testing
make test             # Run unit tests
make test-coverage    # Run with coverage

# Utilities
make fmt              # Format code
make lint             # Run linters
```

### Frontend Commands (`apps/web`)

```bash
cd apps/web

# Development
npm run dev           # Start development server

# Build & Production
npm run build         # Build for production
npm run start         # Start production server

# Code Quality
npm run lint          # Run ESLint
```

---

## 🏗️ Project Architecture

### Backend Architecture

```
apps/api/
├── api/              # API initialization
├── app/              # Application setup
├── cmd/              # CLI commands (seed, etc.)
├── config/           # Configuration management
├── database/         # Database layer & migrations
├── handlers/         # HTTP request handlers
├── model/            # Database models (14 models)
├── router/           # Route definitions
├── services/         # Business logic
│   ├── cron/        # Background jobs
│   └── digitalocean/# AI integration
└── utils/            # Utilities & middleware
```

### Frontend Architecture

```
apps/web/
├── src/
│   ├── app/          # Next.js App Router pages
│   ├── components/   # React components
│   ├── lib/          # Utilities
│   └── types/        # TypeScript types
├── public/           # Static assets
└── services/         # API client services
```

---

## 📊 Features

### Core Features
✅ User Authentication & Authorization  
✅ University & Course Management  
✅ Subject Management with AI  
✅ Document Upload & Management  
✅ AI-Powered Chat Interface  
✅ Real-time Analytics  
✅ Admin Dashboard  
✅ API Key Management  
✅ Background Job Processing  

### AI Features (DigitalOcean)
✅ Automatic Document Indexing  
✅ Knowledge Base Creation  
✅ AI Chat Agents  
✅ Contextual Responses  
✅ Token Usage Tracking  

### Admin Features
✅ User Management  
✅ System Analytics  
✅ Audit Logging  
✅ Settings Management  
✅ API Key Monitoring  

---

## 🗄️ Database Models

1. **User** - User accounts with roles
2. **University** - Educational institutions
3. **Course** - Academic programs
4. **Semester** - Academic terms
5. **Subject** - Course subjects with AI
6. **Document** - Study materials
7. **ChatSession** - Chat conversations
8. **ChatMessage** - Individual messages
9. **ExternalAPIKey** - API keys
10. **APIKeyUsageLog** - API usage tracking
11. **UserActivity** - Activity logs
12. **AdminAuditLog** - Admin actions
13. **AppSetting** - Configuration
14. **JWTTokenBlacklist** - Revoked tokens

---

## 🧪 Testing

### Backend Tests
```bash
cd apps/api
make test              # Unit tests
make test-integration  # Integration tests
make test-coverage     # Coverage report
```

### Frontend Tests
```bash
cd apps/web
npm test              # Run tests
npm run test:watch    # Watch mode
```

---

## 🚢 Deployment

### Docker Deployment (Recommended)

```bash
# Build and run entire stack
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

### Manual Deployment

#### Backend
```bash
cd apps/api
make build
# Deploy bin/server to your server
```

#### Frontend
```bash
cd apps/web
npm run build
# Deploy .next/ directory to Vercel/Netlify or use standalone mode
```

---

## 📈 API Documentation

### Base URL
`http://localhost:8080/api/v1`

### Total Endpoints: 96

**Categories:**
- Auth (5)
- Universities (6)
- Courses (6)
- Semesters (5)
- Subjects (8)
- Documents (7)
- Chat (7)
- Analytics (10)
- API Keys (8)
- Admin (29)
- Health (1)

---

## 🔒 Security

- ✅ JWT-based authentication
- ✅ bcrypt password hashing
- ✅ AES-256 API key encryption
- ✅ Rate limiting (Redis-based)
- ✅ CORS configuration
- ✅ SQL injection protection (GORM)
- ✅ XSS protection
- ✅ Audit logging for admin actions

---

## 📝 License

[Add your license here]

---

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request


