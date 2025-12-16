# AGENTS.md - Study in Woods Monorepo

## Package Manager
- **ALWAYS use `bun`** for installing packages in the web app
- Example: `cd apps/web && bun add package-name`
- For CLI tools: `bunx tool-name@latest`

## Build/Lint/Test Commands
- **Monorepo**: `bun run build`, `bun run lint`, `bun run test` (uses Turborepo)
- **Web (Next.js)**: `bun run web:dev`, `cd apps/web && bun run lint`
- **API (Go/Fiber)**: `cd apps/api && make test` (unit), `make test-integration`, `make lint`
- **Single Go test**: `cd apps/api && go test -v -run TestName ./path/to/package`
- **Docker**: `bun run docker:up`, `make db-up` (PostgreSQL + Redis)

## Code Style
- **Web**: TypeScript + React 19, Next.js 15 App Router, TailwindCSS v4, Radix UI, React Query
- **API**: Go 1.24, Fiber v2, GORM, validator/v10. Run `make fmt` before commits
- **Imports**: Group stdlib, external, internal. Use absolute paths (`@/lib/...` in web)
- **Types**: Strict TypeScript, Zod for validation. Go: use `validator` struct tags
- **Naming**: camelCase (TS), PascalCase components, snake_case (Go files), camelCase (Go vars)
- **Error handling**: Structured errors with status codes. See `apps/web/src/lib/api/client.ts`
- **Cursor rules**: See `.cursor/rules/` - rules auto-apply via globs. Update when patterns emerge

## Project Structure
- `apps/web` - Next.js frontend with `src/` (app router, components, lib, providers, types)
- `apps/api` - Go backend with handlers/, services/, model/, database/
- `apps/ocr-service` - Python FastAPI OCR microservice
