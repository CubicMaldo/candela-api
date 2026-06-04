# Nomad API — Context for Claude Code

## What this is
Location-based social discovery app. Users check in to physical places,
earn points, build reputation. Includes strict safety layer for minors (15-17).

## Stack
Go + Gin, PostgreSQL + PostGIS, Redis, JWT auth, golang-migrate

## Project status
Skeleton only. Core business logic not implemented yet.
See docs/ for full functional specs and implementation plan.

## Key business rules (non-negotiable)
- Users under 15: registration blocked, no data stored
- Users 15-17: Protected Mode, parental consent required before any action
- Check-in validation is SERVER-SIDE only, never trust client GPS
- Places marked apto_menores=PENDING treated as NO (restricted)

## Commands
make run       # start server
make migrate   # run pending migrations
make test      # run tests
make lint      # golangci-lint

## Folder structure
internal/domain/     # pure business entities (no DB tags)
internal/repo/       # SQL queries
internal/service/    # business logic
internal/handler/    # HTTP handlers
internal/web/        # router + middleware
db/migrations/       # golang-migrate SQL files