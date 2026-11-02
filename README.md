# Candela API

REST backend built in **Go** for **Candela**, the social and gamification platform focused on nightlife, outings, and urban hangouts in Colombia (with primary support for Barranquilla and Bogotá).

The API lets users discover physical places, coordinate plans, interact with friends through real-time activity feeds, perform in-person check-ins verified by GPS and photos, earn points, maintain weekly streaks, and compete in city rankings.

---

## 📋 Table of Contents

- [Tech Stack](#-tech-stack)
- [Project Architecture](#-project-architecture)
- [Directory Structure](#-directory-structure)
- [Data Model](#-data-model)
- [Business Flows and Logic](#-business-flows-and-logic)
  - [1. Two-Phase Check-In (Anti-Fraud and Liveness)](#1-two-phase-check-in-anti-fraud-and-liveness)
  - [2. Three-Layer Activity Feed](#2-three-layer-activity-feed)
  - [3. Social Plans and Map Discovery](#3-social-plans-and-map-discovery)
  - [4. Gamification, Streaks, and Ranks](#4-gamification-streaks-and-ranks)
  - [5. Minor Protection](#5-minor-protection)
- [Security and Resilience](#-security-and-resilience)
- [API Reference (Endpoints)](#-api-reference-endpoints)
- [Installation and Setup](#-installation-and-setup)
  - [Requirements](#requirements)
  - [Getting Started](#getting-started)
  - [Useful Commands (Makefile)](#useful-commands-makefile)
- [Environment Variables](#-environment-variables)

---

## 🛠 Tech Stack

| Component | Technology | Use in Candela |
| :--- | :--- | :--- |
| **Language** | **Go 1.26** | Server core, strict typing, and high performance. |
| **Web Framework** | **Gin** (`github.com/gin-gonic/gin`) | HTTP routing, middlewares, payload deserialization and validation. |
| **Database** | **PostgreSQL 16 + PostGIS** | Relational storage and geospatial computation (`GEOGRAPHY(POINT, 4326)`). |
| **SQL Generator** | **sqlc** + **pgx/v5** | Type-safe Go code generation from native SQL queries. |
| **Cache and Concurrency** | **Redis 7** | Per-IP rate limiting, check-in nonces (90s), JWT blocklist, and ranking cache. |
| **Object Storage** | **MinIO (S3 Compatible)** | Check-in photo bucket managed through presigned URLs (`PUT` and `GET`). |
| **Authentication** | **JWT (`v5`) + Google OAuth** | 15-minute access tokens, rotating refresh tokens, and login via Google ID Token. |
| **Migrations** | **golang-migrate/v4** | Version control of the database schema in SQL. |
| **Development Environment** | **Docker Compose + Air** | Container orchestration and hot-reloading during development. |

---

## 🏛 Project Architecture

Candela API follows a **Layered / Clean Architecture**, with strictly separated responsibilities:

```mermaid
flowchart TD
    Client["Client (Mobile App / Web)"] --> HTTP["Gin Router & Middlewares\n(RateLimiter, AuthMiddleware)"]
    HTTP --> Handlers["internal/handler\n(HTTP Controllers / DTOs)"]
    Handlers --> Services["internal/service\n(Use Cases and Business Logic)"]
    Services --> Domain["internal/domain\n(Pure Models, Streak Rules, Cursors)"]
    Services --> Repos["internal/repo\n(Postgres / sqlc / PostGIS Queries)"]
    Services --> Infra["internal/infra\n(Redis, MinIO Storage, PG Pool)"]
    Services -.-> Janitor["Background Janitor\n(Periodic cleanup worker)"]
```

1. **`internal/domain`**: Core entities and pure business logic independent of infrastructure or frameworks (e.g., weekly streak rules, feed cursor decoding, age calculation).
2. **`internal/repo`**: Data access and persistence. Combines `sqlc`-generated code with optimized queries on `pgxpool`.
3. **`internal/service`**: Use-case orchestration (authentication, check-ins, relationships, plans, rankings).
4. **`internal/handler`**: HTTP adapters responsible for parsing requests, verifying contracts, and emitting standardized JSON responses.
5. **`internal/web`**: Router, middleware, and rate-limiting configuration.
6. **`internal/infra`**: Low-level clients for PostgreSQL, Redis, and MinIO S3.

---

## 📂 Directory Structure

```text
candela-api/
├── cmd/
│   ├── server/main.go          # Application entry point and graceful shutdown
│   └── migrate/main.go         # CLI for running migrations with golang-migrate
├── db/
│   ├── migrations/             # SQL migration files (up/down)
│   ├── queries/                # Plain SQL queries for sqlc compilation
│   └── seed_mockdata.sql       # Development test data (Barranquilla)
├── internal/
│   ├── config/                 # Environment variable loading and validation
│   ├── domain/                 # Domain models and pure algorithms
│   ├── handler/                # Gin HTTP controllers
│   ├── infra/                  # Connections: postgres (pgxpool), redis, minio
│   ├── repo/                   # Persistence repositories and sqlcgen
│   ├── service/                # Business logic and Janitor worker
│   ├── web/                    # Route definitions and middlewares
│   └── webdeps/                # Application dependency container
├── tests/                      # Modular test suite (unit, integration, and security)
│   ├── auth/                   # Authentication, JWT, rotation, and revocation
│   ├── checkin/                # Check-in lifecycle and validations
│   ├── domain/                 # Entities and pure business logic
│   ├── dos/                    # Slowloris protection, payload limits, and thundering herd
│   ├── integration/            # Integration tests with PostgreSQL and PostGIS
│   ├── janitor/                # Background tasks and photo/session cleanup
│   ├── plans/                  # Plan lifecycle and visibility
│   ├── privacy/                # Privacy modes and user blocking
│   ├── race/                   # Race conditions (check-in concurrency and rate limit)
│   ├── spoofing/               # Path traversal, IP spoofing, and token tampering
│   └── testhelper/             # Mocks (ObjectStorage) and database/Redis helpers
├── Dockerfile                  # Production deployment image
├── Dockerfile.dev              # Development image with Air
├── docker-compose.yml          # Postgres, PostGIS, Redis, MinIO, and API
├── Makefile                    # Automation and administration commands
├── sqlc.yaml                   # sqlc generator configuration
└── air.toml                    # Air hot-reloading configuration
```

---

## 🗄 Data Model

The relational schema is managed in PostgreSQL with the `postgis` and `uuid-ossp` extensions:

* **`users`**: User identity, hashed password (bcrypt), date of birth, account status (`PENDING`, `ACTIVE`, `SUSPENDED`, `DELETED`), privacy mode (`PUBLIC`, `FRIENDS`, `PRIVATE`), legal guardian support for minors, total/season point counters, and streak state.
* **`businesses`**: Companies and commercial operators (`plan`: `FREE`, `BASIC`, `PREMIUM`).
* **`places`**: Physical places with categories (`RESTAURANT`, `CAFE`, `BAR`, `NIGHTCLUB`, `PARK`, `STORE`, `UNIVERSITY`, etc.), minor suitability (`minor_suitability`: `YES`, `NO`, `PENDING`), the geospatial column `location GEOGRAPHY(POINT, 4326)`, and geofence radius (`geofence_meters` between 20m and 150m, default 80m, validated according to organizer type: user up to 80m, business up to 120m, event up to 150m).
* **`check_ins`**: In-person record with real coordinates, photo (`photo_url`, `photo_key`, `photo_expires_at`, `photo_is_permanent`), geofence validation, and a restriction of **one successful check-in per place per day** (`idx_checkins_one_per_place_per_day`, evaluated according to the `America/Bogota` time zone).
* **`plans` & `plan_participants`**: Social plans with deadlines of 24, 48, or 72 hours. Supports individual mode (`INVITE`) or open mode (`FRIENDS`, `PUBLIC`).
* **`friendships`**: Bidirectional friendships with canonical ordering (`user_id_a < user_id_b`) and block support (`status`: `pending`, `accepted`, `blocked`).
* **`follows`**: Favorite places the user follows to receive updates.
* **`point_transactions`**: Audit trail of points awarded for check-ins, plans, or referrals.
* **`reports`**: Content reporting system for photos, users, or places.
* **`refresh_tokens`**: Persistent mobile sessions with a SHA-256 cryptographic hash (`token_hash`), transactional rotation, and token reuse detection (cascading revocation of all the user's active sessions per RFC 6819 upon malicious replays).

---

## 💡 Business Flows and Logic

### 1. Two-Phase Check-In (Anti-Fraud and Liveness)

To certify that the user is actually present at the place and to avoid gallery photos or spoofing:

```mermaid
sequenceDiagram
    autonumber
    actor C as Mobile Client
    participant A as Candela API
    participant R as Redis
    participant DB as Postgres + PostGIS
    participant S3 as MinIO (S3)

    C->>A: POST /places/:id/checkin/initiate (lat, lon)
    A->>DB: Validate age vs minor_suitability
    A->>DB: Validate geofence (ST_DWithin in meters)
    A->>DB: Validate no check-in there today
    A->>R: Store temporary session (nonce, lat, lon) [TTL: 90s]
    A->>S3: Generate presigned PUT URL
    A-->>C: 200 OK (nonce, upload_url, photo_key)

    C->>S3: Upload photo directly (PUT upload_url)

    C->>A: POST /places/:id/checkin/confirm (nonce, photo_key, permanent?)
    A->>A: Validate path traversal and user isolation (checkins/{user_id}/)
    A->>R: Atomically get and DELETE nonce (GetDel, prevents races)
    A->>DB: Insert check_in (with verified initial coordinates)
    A->>DB: Update weekly streak and compute multipliers
    A->>DB: Award base points, bonuses, and complete pending plans
    A-->>C: 200 OK (checkin, photo_view_url 24h, points breakdown)
```

* **Atomic, Race-Free Consumption (`GetDel`)**: The temporary session is retrieved and deleted from Redis in a single atomic operation (`GetDel`), ensuring strict single use, preventing concurrent duplicate check-ins, and preventing a failed concurrent request from deleting the winning request's photo.
* **Path Traversal and Impersonation Protection**: The server rejects any `photo_key` containing directory traversal sequences (`..`, `\`, `%2e`) and enforces that it must start with the authenticated user's canonical prefix (`checkins/<user_id>/`).
* **Ephemeral vs. Permanent Photos**: Photos expire after 24 hours by default and are deleted by the *Janitor*. The user can mark them as permanent while the event is still active (last 4 hours).

---

### 2. Three-Layer Activity Feed

The feed (`GET /api/feed`) consolidates check-ins with photos from the **last 4 hours**, organized in layers of decreasing relevance:

* **Layer 1 ("Your people")**: Your own check-ins and those of confirmed friends.
* **Layer 2**: Check-ins at places the user follows.
* **Layer 3**: General public activity in the city (excluding private profiles).

**Opaque Cursor Pagination**:
The query uses `DISTINCT ON (checkin_id)` in PostgreSQL so that an event qualifying in several layers is shown only in its highest-priority layer. Pagination is resolved through a Base64 URL-safe token: `FeedCursor` with the format:
$$\text{Base64}(\text{Layer} \mid \text{CheckedInAt} \mid \text{EventID})$$

---

### 3. Social Plans and Map Discovery

* **1-to-1 (`INVITE`)**: Direct invitation to a friend to go to a place within the next 24, 48, or 72 hours.
* **Open (`PUBLIC` or `FRIENDS`)**: Group plans that appear as pins on the map within a radius in kilometers (`GET /api/plans/discover?lat=...&lon=...&radius_km=...`). Any authorized person can join (`POST /plans/:id/join`).
* **Automatic Completion**: If a user with an accepted or active plan checks in at the agreed place, the system automatically completes the plan (`status: COMPLETED`) and credits the plan's bonus points to the participants.

---

### 4. Gamification, Streaks, and Ranks

* **Points**:
  * Base per check-in: +10 pts.
  * First visit of the day: +5 pts.
  * Check-in with photo: +5 pts.
  * Weekly multiplier: $\times 1.5$ (from the second check-in of the week) and $\times 2.0$ (from the fourth).
  * Weekly consistency bonus: +5 or +10 pts for sustaining streaks of 4 or 8+ consecutive weeks.
* **Badge Levels (`badge_level`)**:
  * **Nuevo en la vuelta** (Newcomer): $< 150$ pts
  * **Callejero** (Street Regular): $\ge 150$ pts
  * **Parroquiano** (Local Patron): $\ge 500$ pts
  * **Teso** (Pro): $\ge 1500$ pts
  * **Leyenda** (Legend): $\ge 5000$ pts
* **Rankings**:
  * **Global**: Only for active users with a `PUBLIC` profile and `ranking_opt_in = true`.
  * **Friends**: Ranking among direct friends. Both are cached in Redis and protected against *thundering herd* / cache stampede through `singleflight` (concurrent requests on cache misses are merged into a single database query).

---

### 5. Minor Protection

* If `birth_date` yields an age $< 18$ years or the user has `is_protected = true`:
  * Their privacy can only be `FRIENDS` or `PRIVATE` (never `PUBLIC`).
  * Check-ins are prohibited at places classified as unsuitable for minors (`minor_suitability = 'NO'`).
  * Supports guardian/legal tutor verification (`guardian_contact`, `guardian_token`).

---

## 🛡 Security and Resilience

1. **Access and Refresh Tokens**:
   * **Access JWT**: Short lifetime (15 minutes) to minimize interception risks.
   * **Refresh Tokens**: Secure 256-bit cryptographic strings (`crypto/rand`). They are stored SHA-256 hashed in the database and rotated atomically in a single transaction on each call to `/auth/refresh`.
   * **Token Reuse Detection (RFC 6819)**: If an already revoked refresh token is reused, the system detects the replay attempt and immediately revokes all of the user's active sessions.
   * **Account Deletion and States**: When an account is deleted, the current JWT is immediately added to a Redis blocklist (`jwt:blocklist:<user_id>`) until its lifetime runs out, and all refresh tokens are revoked in the database. Requests with tokens from `SUSPENDED` or `DELETED` accounts are blocked with `403 Forbidden`.
2. **Atomic and Resilient Rate Limiting**:
   * Implemented with **Lua scripts in Redis** (`INCR` + `PEXPIRE` / TTL self-healing), guaranteeing atomicity and preventing race conditions or keys without expiration.
   * Brute-force-sensitive endpoints (`/register`, `/login`): **10 requests/minute**.
   * Session maintenance endpoints (`/refresh`, `/logout`): **120 requests/minute** (to avoid blocking users connected through public Wi-Fi or campus networks with shared NAT).
3. **DoS and Slowloris Mitigation**:
   * **Server HTTP Timeouts**: Server configured with `ReadHeaderTimeout` (5s) to mitigate Slowloris attacks, in addition to `ReadTimeout` (15s), `WriteTimeout` (30s), `IdleTimeout` (60s), and `MaxHeaderBytes` (1MB).
   * **Request Size Limit (Payload Bomb Protection)**: Global `MaxBodySize` middleware set to 1 MB that rejects oversized request bodies (`413 Request Entity Too Large`) and wraps the reader with `http.MaxBytesReader` to protect against memory exhaustion (OOM).
   * **Thundering Herd Protection**: `singleflight` integration in ranking queries to merge concurrent requests when the Redis cache expires.
4. **Spoofing and Path Traversal Protection**:
   * **IP Spoofing**: Strict trusted-proxy configuration in Gin through the `TRUSTED_PROXIES` environment variable. If not specified, any arbitrary `X-Forwarded-For` header is ignored and only the actual remote address (`RemoteAddr`) is trusted.
   * **Path Traversal & Photo Key Isolation**: Defensive validation against escape sequences (`..`, `\`, `%2e`) and requirement of the `checkins/<user_id>/` prefix on confirmed photos.
   * **Atomic Session Consumption (`GetDel`)**: Atomic retrieval and deletion of the nonce in Redis to avoid concurrent race conditions in check-ins.
5. **Concurrent Janitor Worker**:
   * Background process that runs periodically every hour.
   * Claims batches of expired photos in the database using `FOR UPDATE SKIP LOCKED`, ensuring that multiple API instances can run concurrently without contending for the same records.
   * Deletes orphaned objects from MinIO and purges expired refresh tokens.

---

## 📡 API Reference (Endpoints)

All authenticated routes require the header: `Authorization: Bearer <access_token>`.

### Authentication and Session
* `POST /api/auth/register` — Traditional registration with email, username, password, and date of birth.
* `POST /api/auth/login` — Login with credentials.
* `POST /api/auth/google` — Login with Google ID Token.
* `POST /api/auth/google/register` — Complete registration for a new Google account.
* `POST /api/auth/refresh` — Refresh token rotation and issuance of a new token pair.
* `POST /api/auth/logout` — Refresh token revocation.

### Profile and User
* `GET /api/me` — Full own profile, streak metrics, and aggregated statistics.
* `PATCH /api/me/privacy` — Update privacy mode (`PUBLIC`, `FRIENDS`, `PRIVATE`).
* `PATCH /api/me/ranking` — Enable or disable inclusion in the global ranking (`ranking_opt_in`).
* `DELETE /api/me` — Delete account, revoke sessions, and delete associated photos.
* `GET /api/me/check-ins` — Own check-in history with time-based pagination.
* `GET /api/me/plans` — Own plan history (filterable by status).
* `GET /api/me/friends` — List of confirmed friends.
* `GET /api/feed` — Three-layer social feed with cursor pagination.

### Places (`/api/places`)
* `GET /api/places/nearby?lat=...&lon=...&radius_km=...&category=...` — Proximity-based place search using PostGIS.
* `GET /api/places/:place_id` — Place details, whether the user follows it, and active concurrency (last 4h).
* `GET /api/places/:place_id/feed` — Live place feed (recent check-ins with photo).
* `POST /api/places/:place_id/follow` — Follow place.
* `DELETE /api/places/:place_id/follow` — Unfollow place.
* `POST /api/places/:place_id/checkin/initiate` — Start check-in (GPS validation + presigned PUT URL).
* `POST /api/places/:place_id/checkin/confirm` — Confirm check-in and award points.
* `PATCH /api/checkins/:checkin_id/permanent` — Turn photo permanent or ephemeral.

### Plans (`/api/plans`)
* `POST /api/plans` — Create plan (1-to-1 or open).
* `GET /api/plans/discover?lat=...&lon=...&radius_km=...` — Discover open plans on the map.
* `GET /api/plans/incoming` — Plans the user was invited to.
* `GET /api/plans/outgoing` — Plans created by the user.
* `GET /api/plans/:plan_id` — Plan details and participants.
* `POST /api/plans/:plan_id/accept` — Accept an invited plan.
* `POST /api/plans/:plan_id/decline` — Decline an invited plan.
* `POST /api/plans/:plan_id/join` — Join an open plan.
* `DELETE /api/plans/:plan_id` — Cancel plan (creator only).

### Users and Friendships
* `GET /api/users?q=...` — Search users by username prefix.
* `GET /api/users/:username` — View public or partial profile depending on friendship and privacy.
* `POST /api/users/:user_id/block` — Block a user.
* `DELETE /api/users/:user_id/block` — Unblock a user.
* `POST /api/users/:user_id/friend-request` — Send friend request.
* `DELETE /api/users/:user_id/friend` — Remove a friend.
* `GET /api/friend-requests/incoming` — Received friend requests.
* `GET /api/friend-requests/outgoing` — Sent friend requests.
* `POST /api/friend-requests/:user_id/accept` — Accept friend request.
* `POST /api/friend-requests/:user_id/reject` — Reject friend request.
* `DELETE /api/friend-requests/:user_id` — Cancel a sent friend request.

### Rankings and Moderation
* `GET /api/rankings/global` — Top global city ranking (Redis cached).
* `GET /api/rankings/friends` — Leaderboard between the user and their friends.
* `POST /api/reports` — Report a photo, a user, or a place.

---

## 🚀 Installation and Setup

### Requirements
* [Docker](https://www.docker.com/) and [Docker Compose](https://docs.docker.com/compose/)
* `make` (optional but recommended)
* Go 1.26+ (if you want to build locally outside the container)

### Getting Started

1. **Configure environment variables**:
   ```bash
   make setup
   # Or manually: cp .env.example .env
   ```
   Review the values in `.env` (ports, Postgres credentials, Redis, etc.).

2. **Start the services**:
   ```bash
   make up
   ```
   This will start:
   * **PostgreSQL + PostGIS**: `localhost:5432`
   * **Redis**: `localhost:6379`
   * **MinIO API**: `localhost:9000` (Web console at `localhost:9001`)
   * **Candela API (with Air)**: `http://localhost:8080`

3. **Run migrations**:
   ```bash
   make migrate-up
   ```

4. **Load test data (Mock Data)**:
   ```bash
   make seed
   ```
   *Inserts test users, businesses, and real places in Barranquilla (shared password: `password123`).*

5. **Check health status**:
   ```bash
   curl http://localhost:8080/health
   # Response: {"env":"development","service":"candela-api","status":"ok"}
   ```

---

### Useful Commands (Makefile)

```bash
make help            # Shows the list of available commands
make up              # Starts all containers in the background
make down            # Stops the containers (preserves volumes)
make down-v          # Stops the containers and DELETES the data
make logs-api        # Follows the API logs live
make shell-api       # Opens a sh terminal inside the API container
make shell-db        # Connects to the database with psql
make redis-cli       # Opens the interactive Redis console
make migrate-up      # Applies pending SQL migrations
make migrate-down    # Reverts the last migration
make sqlc-generate   # Regenerates repository code from db/queries/
make test            # Runs the test suite (unit, integration, concurrency, DoS, spoofing)
```

---

## ⚙️ Environment Variables

Defined in the `.env` file:

| Variable | Description | Default Value / Example |
| :--- | :--- | :--- |
| `APP_ENV` | Runtime environment (`development`, `production`). | `development` |
| `PORT` | HTTP port exposed by the API. | `8080` |
| `APP_TIMEZONE` | IANA time zone for the day window in nightlife. | `America/Bogota` |
| `DATABASE_URL` | PostgreSQL connection string. | `postgres://candela_user:...@localhost:5432/candela_db?sslmode=disable` |
| `REDIS_URL` | Redis connection with password. | `redis://:candela_redis_pass@localhost:6379/0` |
| `JWT_SECRET` | Secret key for signing JWT tokens (minimum 32 characters in production). | `insecure-dev-only-secret` |
| `GOOGLE_CLIENT_ID`| Google OAuth Client ID to validate mobile ID Tokens. | Optional in development |
| `TRUSTED_PROXIES` | Comma-separated IPs or CIDR ranges of trusted reverse proxies for Gin (mitigates IP spoofing). | Optional (by default trusts only `RemoteAddr`) |
| `STORAGE_ENDPOINT`| MinIO service URL for the internal API. | `http://minio:9000` |
| `STORAGE_PUBLIC_ENDPOINT`| URL reachable by mobile devices/emulators (e.g., Android loopback). | `http://10.0.2.2:9000` |
| `STORAGE_ACCESS_KEY` | MinIO S3 user/key. | `minioadmin` |
| `STORAGE_SECRET_KEY` | MinIO S3 secret password. | `minioadmin` |
| `STORAGE_BUCKET` | Bucket name for storing photos. | `candela-photos` |
| `STORAGE_USE_SSL` | Enable SSL for the MinIO client (`true`/`false`). | `false` |
