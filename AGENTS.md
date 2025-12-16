# AGENTS.md - Study in Woods Monorepo

## Build/Lint/Test Commands
- **Monorepo**: `npm run build`, `npm run lint`, `npm run test` (uses Turborepo)
- **Web (Next.js)**: `npm run web:dev`, `cd apps/web && npm run lint`
- **API (Go/Fiber)**: `cd apps/api && make test` (unit), `make test-integration`, `make lint`
- **Single Go test**: `cd apps/api && go test -v -run TestName ./path/to/package`
- **Docker**: `npm run docker:up`, `make db-up` (PostgreSQL + Redis)

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
