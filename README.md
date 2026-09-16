# Candela API

Backend REST desarrollado en **Go** para **Candela**, la plataforma social y de gamificación orientada a la vida nocturna, salidas y parches urbanos en Colombia (con soporte principal para Barranquilla y Bogotá).

La API permite a los usuarios descubrir lugares físicos, coordinar planes, interactuar con amigos mediante feeds de actividad en tiempo real, realizar check-ins presenciales verificados por GPS y fotos, ganar puntos, mantener rachas semanales y competir en rankings de ciudad.

---

## 📋 Tabla de Contenidos

- [Stack Tecnológico](#-stack-tecnológico)
- [Arquitectura del Proyecto](#-arquitectura-del-proyecto)
- [Estructura de Directorios](#-estructura-de-directorios)
- [Modelo de Datos](#-modelo-de-datos)
- [Flujos y Lógica de Negocio](#-flujos-y-lógica-de-negocio)
  - [1. Check-In en 2 Fases (Anti-Fraude y Liveness)](#1-check-in-en-2-fases-anti-fraude-y-liveness)
  - [2. Feed de Actividad en 3 Capas](#2-feed-de-actividad-en-3-capas)
  - [3. Planes Sociales y Descubrimiento en Mapa](#3-planes-sociales-y-descubrimiento-en-mapa)
  - [4. Gamificación, Rachas y Rangos](#4-gamificación-rachas-y-rangos)
  - [5. Protección de Menores de Edad](#5-protección-de-menores-de-edad)
- [Seguridad y Resiliencia](#-seguridad-y-resiliencia)
- [Referencia de la API (Endpoints)](#-referencia-de-la-api-endpoints)
- [Instalación y Puesta en Marcha](#-instalación-y-puesta-en-marcha)
  - [Requisitos](#requisitos)
  - [Primeros Pasos](#primeros-pasos)
  - [Comandos Útiles (Makefile)](#comandos-útiles-makefile)
- [Variables de Entorno](#-variables-de-entorno)

---

## 🛠 Stack Tecnológico

| Componente | Tecnología | Uso en Candela |
| :--- | :--- | :--- |
| **Lenguaje** | **Go 1.26** | Núcleo del servidor, tipado estricto y alto rendimiento. |
| **Framework Web** | **Gin** (`github.com/gin-gonic/gin`) | Ruteo HTTP, middlewares, deserialización y validación de payloads. |
| **Base de Datos** | **PostgreSQL 16 + PostGIS** | Almacenamiento relacional y cálculo geoespacial (`GEOGRAPHY(POINT, 4326)`). |
| **Generador SQL** | **sqlc** + **pgx/v5** | Generación de código Go type-safe a partir de queries SQL nativas. |
| **Caché y Concurrencia** | **Redis 7** | Rate limiting por IP, nonces de check-in (90s), blocklist de JWT y caché de rankings. |
| **Almacenamiento de Objetos** | **MinIO (S3 Compatible)** | Bucket de fotos de check-in gestionado mediante URLs prefirmadas (`PUT` y `GET`). |
| **Autenticación** | **JWT (`v5`) + Google OAuth** | Tokens de acceso de 15 min, tokens de refresco rotativos y login vía Google ID Token. |
| **Migraciones** | **golang-migrate/v4** | Control de versiones del esquema de base de datos en SQL. |
| **Entorno de Desarrollo** | **Docker Compose + Air** | Orquestación de contenedores y hot-reloading en tiempo de desarrollo. |

---

## 🏛 Arquitectura del Proyecto

Candela API sigue una **Arquitectura en Capas / Clean Architecture**, separando estrictamente responsabilidades:

```mermaid
flowchart TD
    Client["Cliente (App Móvil / Web)"] --> HTTP["Gin Router & Middlewares\n(RateLimiter, AuthMiddleware)"]
    HTTP --> Handlers["internal/handler\n(Controladores HTTP / DTOs)"]
    Handlers --> Services["internal/service\n(Casos de Uso y Lógica de Negocio)"]
    Services --> Domain["internal/domain\n(Modelos Puros, Reglas de Racha, Cursors)"]
    Services --> Repos["internal/repo\n(Postgres / sqlc / PostGIS Queries)"]
    Services --> Infra["internal/infra\n(Redis, MinIO Storage, PG Pool)"]
    Services -.-> Janitor["Background Janitor\n(Worker periódico de limpieza)"]
```

1. **`internal/domain`**: Entidades centrales y lógica pura de negocio independiente de infraestructura o frameworks (e.g., reglas de rachas semanales, decodificación de cursores de feed, cálculo de edad).
2. **`internal/repo`**: Acceso a datos y persistencia. Combina código generado por `sqlc` con queries optimizadas en `pgxpool`.
3. **`internal/service`**: Orquestación de casos de uso (autenticación, check-ins, relaciones, planes, ranking).
4. **`internal/handler`**: Adaptadores HTTP encargados de parsear las peticiones, verificar contratos y emitir respuestas JSON estandarizadas.
5. **`internal/web`**: Configuración de routers, middlewares y rate-limiting.
6. **`internal/infra`**: Clientes de bajo nivel para PostgreSQL, Redis y MinIO S3.

---

## 📂 Estructura de Directorios

```text
candela-api/
├── cmd/
│   ├── server/main.go          # Punto de entrada de la aplicación y graceful shutdown
│   └── migrate/main.go         # CLI para correr migraciones con golang-migrate
├── db/
│   ├── migrations/             # Archivos SQL de migración (up/down)
│   ├── queries/                # Queries SQL puras para compilación con sqlc
│   └── seed_mockdata.sql       # Datos de prueba para desarrollo (Barranquilla)
├── internal/
│   ├── config/                 # Carga de variables de entorno y validaciones
│   ├── domain/                 # Modelos de dominio y algoritmos puros
│   ├── handler/                # Controladores HTTP de Gin
│   ├── infra/                  # Conexiones: postgres (pgxpool), redis, minio
│   ├── repo/                   # Repositorios de persistencia y sqlcgen
│   ├── service/                # Lógica de negocio y worker Janitor
│   ├── web/                    # Definición de rutas y middlewares
│   └── webdeps/                # Contenedor de dependencias de la aplicación
├── tests/                      # Suite modular de pruebas (unitarias, integración y seguridad)
│   ├── auth/                   # Autenticación, JWT, rotación y revocación
│   ├── checkin/                # Ciclo de vida y validaciones de check-in
│   ├── domain/                 # Entidades y lógica pura de negocio
│   ├── dos/                    # Protección Slowloris, payload limits y thundering herd
│   ├── integration/            # Pruebas de integración con PostgreSQL y PostGIS
│   ├── janitor/                # Tareas en segundo plano y limpieza de fotos/sesiones
│   ├── plans/                  # Ciclo de vida y visibilidad de planes
│   ├── privacy/                # Modos de privacidad y bloqueo de usuarios
│   ├── race/                   # Condiciones de carrera (concurrencia de check-in y rate limit)
│   ├── spoofing/               # Path traversal, IP spoofing y alteración de tokens
│   └── testhelper/             # Mocks (ObjectStorage) y helpers de base de datos/Redis
├── Dockerfile                  # Imagen para despliegue de producción
├── Dockerfile.dev              # Imagen de desarrollo con Air
├── docker-compose.yml          # Postgres, PostGIS, Redis, MinIO y API
├── Makefile                    # Comandos de automatización y administración
├── sqlc.yaml                   # Configuración del generador sqlc
└── air.toml                    # Configuración de hot-reloading de Air
```

---

## 🗄 Modelo de Datos

El esquema relacional se gestiona en PostgreSQL con extensiones `postgis` y `uuid-ossp`:

* **`users`**: Identidad del usuario, contraseña hasheada (bcrypt), fecha de nacimiento, estado de cuenta (`PENDING`, `ACTIVE`, `SUSPENDED`, `DELETED`), modo de privacidad (`PUBLIC`, `FRIENDS`, `PRIVATE`), soporte de tutor legal para menores, contadores de puntos totales/temporada y estado de rachas.
* **`businesses`**: Empresas y operadores comerciales (`plan`: `FREE`, `BASIC`, `PREMIUM`).
* **`places`**: Lugares físicos con categorías (`RESTAURANT`, `CAFE`, `BAR`, `NIGHTCLUB`, `PARK`, `STORE`, `UNIVERSITY`, etc.), aptitud para menores (`minor_suitability`: `YES`, `NO`, `PENDING`), columna geoespacial `location GEOGRAPHY(POINT, 4326)` y radio de geocerca (`geofence_meters` entre 20m y 150m, por defecto 80m, validado según el tipo de organizador: usuario hasta 80m, negocio hasta 120m, evento hasta 150m).
* **`check_ins`**: Registro presencial con coordenadas reales, foto (`photo_url`, `photo_key`, `photo_expires_at`, `photo_is_permanent`), validación de geocerca y restricción de **un solo check-in exitoso por lugar por día** (`idx_checkins_one_per_place_per_day` evaluado según la zona `America/Bogota`).
* **`plans` & `plan_participants`**: Planes sociales con plazos de 24, 48 o 72 horas. Soporta modo individual (`INVITE`) o abiertos (`FRIENDS`, `PUBLIC`).
* **`friendships`**: Amistades bidireccionales con orden canónico (`user_id_a < user_id_b`) y soporte de bloqueos (`status`: `pending`, `accepted`, `blocked`).
* **`follows`**: Lugares favoritos que el usuario sigue para recibir actualizaciones.
* **`point_transactions`**: Auditoría de puntos otorgados por check-ins, planes o referidos.
* **`reports`**: Sistema de denuncias de contenido sobre fotos, usuarios o lugares.
* **`refresh_tokens`**: Sesiones móviles persistentes con hash criptográfico SHA-256 (`token_hash`), rotación transaccional y detección de reuso de tokens (revocación en cascada de todas las sesiones activas del usuario según RFC 6819 ante repeticiones maliciosas).

---

## 💡 Flujos y Lógica de Negocio

### 1. Check-In en 2 Fases (Anti-Fraude y Liveness)

Para certificar que el usuario realmente está presente en el lugar y evitar fotos de galería o spoofing:

```mermaid
sequenceDiagram
    autonumber
    actor C as Cliente Móvil
    participant A as Candela API
    participant R as Redis
    participant DB as Postgres + PostGIS
    participant S3 as MinIO (S3)

    C->>A: POST /places/:id/checkin/initiate (lat, lon)
    A->>DB: Validar edad vs minor_suitability
    A->>DB: Validar geofence (ST_DWithin en metros)
    A->>DB: Validar que no haya hecho check-in hoy allí
    A->>R: Guardar sesión temporal (nonce, lat, lon) [TTL: 90s]
    A->>S3: Generar presigned PUT URL
    A-->>C: 200 OK (nonce, upload_url, photo_key)

    C->>S3: Subir foto directamente (PUT upload_url)

    C->>A: POST /places/:id/checkin/confirm (nonce, photo_key, permanent?)
    A->>A: Validar path traversal y aislamiento de usuario (checkins/{user_id}/)
    A->>R: Obtener y ELIMINAR nonce atómicamente (GetDel, previene carreras)
    A->>DB: Insertar check_in (con coords iniciales verificadas)
    A->>DB: Actualizar racha semanal y calcular multiplicadores
    A->>DB: Otorgar puntos base, bonus y completar planes pendientes
    A-->>C: 200 OK (checkin, photo_view_url 24h, breakdown de puntos)
```

* **Consumo Atómico y Antirace (`GetDel`)**: La sesión temporal se extrae y elimina de Redis en una única operación atómica (`GetDel`), asegurando uso único estricto, previniendo check-ins duplicados concurrentes y evitando que una petición concurrente fallida elimine la foto de la petición ganadora.
* **Protección contra Path Traversal y Suplantación**: El servidor rechaza cualquier `photo_key` con secuencias de salto de directorio (`..`, `\`, `%2e`) y fuerza que comience obligatoriamente con el prefijo canónico del usuario autenticado (`checkins/<user_id>/`).
* **Fotos efímeras vs permanentes**: Las fotos expiran por defecto en 24 horas y son borradas por el *Janitor*. El usuario puede marcarlas como permanentes mientras el evento siga activo (últimas 4 horas).

---

### 2. Feed de Actividad en 3 Capas

El feed (`GET /api/feed`) consolida los check-ins con fotos ocurridos en las **últimas 4 horas**, organizados en capas de relevancia decreciente:

* **Capa 1 ("Tu gente")**: Check-ins propios y de amigos confirmados.
* **Capa 2**: Check-ins en lugares que el usuario sigue.
* **Capa 3**: Actividad pública general en la ciudad (excluyendo perfiles privados).

**Paginación por Cursor Opaco**:
La query utiliza `DISTINCT ON (checkin_id)` en PostgreSQL para que un evento que califique en varias capas se muestre únicamente en su capa de mayor prioridad. El paginado se resuelve mediante un token Base64 URL-safe: `FeedCursor` con formato:
$$\text{Base64}(\text{Layer} \mid \text{CheckedInAt} \mid \text{EventID})$$

---

### 3. Planes Sociales y Descubrimiento en Mapa

* **1 a 1 (`INVITE`)**: Invitación directa a un amigo para ir a un lugar en las próximas 24, 48 o 72 horas.
* **Abiertos (`PUBLIC` o `FRIENDS`)**: Planes grupales que aparecen como pines en el mapa dentro de un radio en kilómetros (`GET /api/plans/discover?lat=...&lon=...&radius_km=...`). Cualquier persona autorizada puede unirse (`POST /plans/:id/join`).
* **Concreción Automática**: Si un usuario con un plan aceptado o activo hace check-in en el lugar acordado, el sistema completa el plan automáticamente (`status: COMPLETED`) y acredita los puntos de bonificación del plan a los participantes.

---

### 4. Gamificación, Rachas y Rangos

* **Puntos**:
  * Base por check-in: +10 pts.
  * Primera visita del día: +5 pts.
  * Check-in con foto: +5 pts.
  * Multiplicador semanal: $\times 1.5$ (a partir del segundo check-in en la semana) y $\times 2.0$ (a partir del cuarto).
  * Bono de consistencia semanal: +5 o +10 pts al sostener rachas de 4 o 8+ semanas consecutivas.
* **Niveles de Insignia (`badge_level`)**:
  * **Nuevo en la vuelta**: $< 150$ pts
  * **Callejero**: $\ge 150$ pts
  * **Parroquiano**: $\ge 500$ pts
  * **Teso**: $\ge 1500$ pts
  * **Leyenda**: $\ge 5000$ pts
* **Rankings**:
  * **Global**: Solo para usuarios activos con perfil `PUBLIC` y `ranking_opt_in = true`.
  * **Amigos**: Ranking entre amigos directos. Ambos con caché en Redis y protección contra *thundering herd* / estampida de caché mediante `singleflight` (las peticiones concurrentes ante fallos de caché se unifican en una sola consulta a la base de datos).

---

### 5. Protección de Menores de Edad

* Si `birth_date` arroja $< 18$ años o el usuario tiene `is_protected = true`:
  * Su privacidad solo puede ser `FRIENDS` o `PRIVATE` (nunca `PUBLIC`).
  * Se prohíbe el check-in en lugares clasificados como no aptos para menores (`minor_suitability = 'NO'`).
  * Cuenta con soporte para verificación de acudiente/tutor legal (`guardian_contact`, `guardian_token`).

---

## 🛡 Seguridad y Resiliencia

1. **Tokens de Acceso y Refresco**:
   * **JWT de acceso**: Tiempo de vida corto (15 minutos) para minimizar riesgos de intercepción.
   * **Refresh Tokens**: Cadenas criptográficas seguras de 256 bits (`crypto/rand`). Se guardan hasheadas con SHA-256 en la base de datos y se rotan atómicamente en una sola transacción en cada llamado a `/auth/refresh`.
   * **Detección de Reuso de Tokens (RFC 6819)**: Si un refresh token ya revocado es reutilizado, el sistema detecta el intento de repetición y revoca inmediatamente todas las sesiones activas del usuario.
   * **Baja de cuenta y Estados**: Al eliminar la cuenta, el JWT actual entra inmediatamente a una lista negra en Redis (`jwt:blocklist:<user_id>`) hasta agotar su tiempo de vida, y todos los refresh tokens se revocan en la base de datos. Peticiones con tokens de cuentas `SUSPENDED` o `DELETED` son bloqueadas con `403 Forbidden`.
2. **Rate Limiting Atómico y Resiliente**:
   * Implementado con scripts **Lua en Redis** (`INCR` + `PEXPIRE` / autoreparación de TTL), garantizando atomicidad y previniendo condiciones de carrera o llaves sin expiración.
   * Endpoints sensibles a fuerza bruta (`/register`, `/login`): **10 peticiones/minuto**.
   * Endpoints de mantenimiento de sesión (`/refresh`, `/logout`): **120 peticiones/minuto** (para evitar bloquear a usuarios conectados a redes Wi-Fi públicas o de campus con NAT compartido).
3. **Mitigación de DoS y Slowloris**:
   * **Timeouts HTTP en Servidor**: Servidor configurado con `ReadHeaderTimeout` (5s) para mitigar ataques Slowloris, además de `ReadTimeout` (15s), `WriteTimeout` (30s), `IdleTimeout` (60s) y `MaxHeaderBytes` (1MB).
   * **Límite de Tamaño de Petición (Payload Bomb Protection)**: Middleware global `MaxBodySize` configurado a 1 MB que rechaza cuerpos de petición desproporcionados (`413 Request Entity Too Large`) y envuelve el reader con `http.MaxBytesReader` para proteger contra agotamiento de memoria (OOM).
   * **Protección contra Thundering Herd**: Integración de `singleflight` en las consultas de rankings para unificar peticiones concurrentes cuando expira la caché de Redis.
4. **Protección contra Spoofing y Path Traversal**:
   * **IP Spoofing**: Configuración estricta de proxies de confianza en Gin mediante la variable de entorno `TRUSTED_PROXIES`. Si no se especifica, se ignora cualquier cabecera `X-Forwarded-For` arbitraria y se confía únicamente en la dirección remota real (`RemoteAddr`).
   * **Path Traversal & Photo Key Isolation**: Validación defensiva contra secuencias de escape (`..`, `\`, `%2e`) y exigencia del prefijo `checkins/<user_id>/` en las fotos confirmadas.
   * **Consumo Atómico de Sesión (`GetDel`)**: Recuperación y eliminación atómica del nonce en Redis para evitar condiciones de carrera concurrentes en check-ins.
5. **Janitor Worker Concurrente**:
   * Proceso en background que corre periódicamente cada hora.
   * Reclama lotes de fotos expiradas en la base de datos utilizando `FOR UPDATE SKIP LOCKED`, garantizando que múltiples instancias de la API puedan correr concurrentemente sin disputar los mismos registros.
   * Borra los objetos huérfanos de MinIO y depura tokens de refresco vencidos.

---

## 📡 Referencia de la API (Endpoints)

Todas las rutas autenticadas requieren el encabezado: `Authorization: Bearer <access_token>`.

### Autenticación y Sesión
* `POST /api/auth/register` — Registro tradicional con email, username, contraseña y fecha de nacimiento.
* `POST /api/auth/login` — Inicio de sesión con credenciales.
* `POST /api/auth/google` — Inicio de sesión con Google ID Token.
* `POST /api/auth/google/register` — Completar registro para cuenta de Google nueva.
* `POST /api/auth/refresh` — Rotación de refresh token y emisión de nuevo par de tokens.
* `POST /api/auth/logout` — Revocación de refresh token.

### Perfil y Usuario
* `GET /api/me` — Perfil propio completo, métricas de racha y estadísticas agregadas.
* `PATCH /api/me/privacy` — Actualizar modo de privacidad (`PUBLIC`, `FRIENDS`, `PRIVATE`).
* `PATCH /api/me/ranking` — Habilitar o deshabilitar inclusión en el ranking global (`ranking_opt_in`).
* `DELETE /api/me` — Eliminar cuenta, revocar sesiones y eliminar fotos asociadas.
* `GET /api/me/check-ins` — Historial propio de check-ins con paginación temporal.
* `GET /api/me/plans` — Historial de planes propios (filtro por estado).
* `GET /api/me/friends` — Listado de amigos confirmados.
* `GET /api/feed` — Feed social en 3 capas con paginación por cursor.

### Lugares (`/api/places`)
* `GET /api/places/nearby?lat=...&lon=...&radius_km=...&category=...` — Búsqueda de lugares por proximidad PostGIS.
* `GET /api/places/:place_id` — Detalle del lugar, si el usuario lo sigue y concurrencia activa (últimas 4h).
* `GET /api/places/:place_id/feed` — Feed en vivo del lugar (check-ins recientes con foto).
* `POST /api/places/:place_id/follow` — Seguir lugar.
* `DELETE /api/places/:place_id/follow` — Dejar de seguir lugar.
* `POST /api/places/:place_id/checkin/initiate` — Iniciar check-in (validación GPS + URL prefirmada PUT).
* `POST /api/places/:place_id/checkin/confirm` — Confirmar check-in y adjudicar puntos.
* `PATCH /api/checkins/:checkin_id/permanent` — Convertir foto en permanente o efímera.

### Planes (`/api/plans`)
* `POST /api/plans` — Crear plan (1 a 1 o abierto).
* `GET /api/plans/discover?lat=...&lon=...&radius_km=...` — Descubrir planes abiertos en el mapa.
* `GET /api/plans/incoming` — Planes donde el usuario fue invitado.
* `GET /api/plans/outgoing` — Planes creados por el usuario.
* `GET /api/plans/:plan_id` — Detalle del plan y participantes.
* `POST /api/plans/:plan_id/accept` — Aceptar plan invitado.
* `POST /api/plans/:plan_id/decline` — Declinar plan invitado.
* `POST /api/plans/:plan_id/join` — Unirse a plan abierto.
* `DELETE /api/plans/:plan_id` — Cancelar plan (solo el creador).

### Usuarios y Amistades
* `GET /api/users?q=...` — Búsqueda de usuarios por prefijo de nombre de usuario.
* `GET /api/users/:username` — Ver perfil público o parcial según relación de amistad y privacidad.
* `POST /api/users/:user_id/block` — Bloquear a un usuario.
* `DELETE /api/users/:user_id/block` — Desbloquear a un usuario.
* `POST /api/users/:user_id/friend-request` — Enviar solicitud de amistad.
* `DELETE /api/users/:user_id/friend` — Eliminar a un amigo.
* `GET /api/friend-requests/incoming` — Solicitudes de amistad recibidas.
* `GET /api/friend-requests/outgoing` — Solicitudes de amistad enviadas.
* `POST /api/friend-requests/:user_id/accept` — Aceptar solicitud de amistad.
* `POST /api/friend-requests/:user_id/reject` — Rechazar solicitud de amistad.
* `DELETE /api/friend-requests/:user_id` — Cancelar solicitud de amistad enviada.

### Rankings y Moderación
* `GET /api/rankings/global` — Top ranking global de la ciudad (con caché Redis).
* `GET /api/rankings/friends` — Tabla de posiciones entre el usuario y sus amigos.
* `POST /api/reports` — Denunciar una foto, un usuario o un lugar.

---

## 🚀 Instalación y Puesta en Marcha

### Requisitos
* [Docker](https://www.docker.com/) y [Docker Compose](https://docs.docker.com/compose/)
* `make` (opcional pero recomendado)
* Go 1.26+ (si deseas compilar localmente fuera del contenedor)

### Primeros Pasos

1. **Configurar variables de entorno**:
   ```bash
   make setup
   # O manualmente: cp .env.example .env
   ```
   Revisa los valores en `.env` (puertos, credenciales de Postgres, Redis, etc.).

2. **Levantar los servicios**:
   ```bash
   make up
   ```
   Esto iniciará:
   * **PostgreSQL + PostGIS**: `localhost:5432`
   * **Redis**: `localhost:6379`
   * **MinIO API**: `localhost:9000` (Consola web en `localhost:9001`)
   * **Candela API (con Air)**: `http://localhost:8080`

3. **Ejecutar migraciones**:
   ```bash
   make migrate-up
   ```

4. **Cargar datos de prueba (Mock Data)**:
   ```bash
   make seed
   ```
   *Inserta usuarios de prueba, negocios y lugares reales de Barranquilla (contraseña general: `password123`).*

5. **Verificar estado de salud**:
   ```bash
   curl http://localhost:8080/health
   # Respuesta: {"env":"development","service":"candela-api","status":"ok"}
   ```

---

### Comandos Útiles (Makefile)

```bash
make help            # Muestra el listado de comandos disponibles
make up              # Levanta todos los contenedores en segundo plano
make down            # Detiene los contenedores (preserva volúmenes)
make down-v          # Detiene los contenedores y BORRA los datos
make logs-api        # Sigue los logs de la API en vivo
make shell-api       # Abre una terminal sh dentro del contenedor de la API
make shell-db        # Conecta con psql en la base de datos
make redis-cli       # Abre la consola interactiva de Redis
make migrate-up      # Aplica migraciones SQL pendientes
make migrate-down    # Revierte la última migración
make sqlc-generate   # Regenera el código de repositorios a partir de db/queries/
make test            # Ejecuta la suite de pruebas (unitarias, integración, concurrencia, DoS, spoofing)
```

---

## ⚙️ Variables de Entorno

Definidas en el archivo `.env`:

| Variable | Descripción | Valor por Defecto / Ejemplo |
| :--- | :--- | :--- |
| `APP_ENV` | Entorno de ejecución (`development`, `production`). | `development` |
| `PORT` | Puerto HTTP expuesto por la API. | `8080` |
| `APP_TIMEZONE` | Zona horaria IANA para la ventana de días en vida nocturna. | `America/Bogota` |
| `DATABASE_URL` | String de conexión a PostgreSQL. | `postgres://candela_user:...@localhost:5432/candela_db?sslmode=disable` |
| `REDIS_URL` | Conexión a Redis con contraseña. | `redis://:candela_redis_pass@localhost:6379/0` |
| `JWT_SECRET` | Llave secreta para firmar tokens JWT (mínimo 32 caracteres en producción). | `insecure-dev-only-secret` |
| `GOOGLE_CLIENT_ID`| Client ID de Google OAuth para validar ID Tokens móviles. | Opcional en desarrollo |
| `TRUSTED_PROXIES` | IPs o rangos CIDR de proxies reversos de confianza separados por comas para Gin (mitiga IP spoofing). | Opcional (por defecto confía únicamente en `RemoteAddr`) |
| `STORAGE_ENDPOINT`| URL del servicio MinIO para la API interna. | `http://minio:9000` |
| `STORAGE_PUBLIC_ENDPOINT`| URL alcanzable por dispositivos móviles/emuladores (e.g., loopback Android). | `http://10.0.2.2:9000` |
| `STORAGE_ACCESS_KEY` | Usuario/Key de MinIO S3. | `minioadmin` |
| `STORAGE_SECRET_KEY` | Contraseña secreta de MinIO S3. | `minioadmin` |
| `STORAGE_BUCKET` | Nombre del bucket para almacenar fotos. | `candela-photos` |
| `STORAGE_USE_SSL` | Activar SSL para el cliente MinIO (`true`/`false`). | `false` |

