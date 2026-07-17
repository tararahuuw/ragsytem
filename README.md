# RAG System — Backend

Backend API untuk aplikasi RAG (NotebookLM-like): upload dokumen → ingest → tanya-jawab.
Dibangun dengan **Go + Gin + GORM**, arsitektur berlapis yang sederhana namun rapi.

## Tech Stack
- **Go 1.26**
- **Gin** — HTTP framework
- **GORM** — ORM (PostgreSQL)
- **JWT** (golang-jwt) — auth + RBAC + multi-tenant (organizationCode)
- **MinIO** (minio-go) — object storage untuk upload file besar (chunked + compose)
- **swaggo/swag** — dokumentasi API (Swagger/OpenAPI)
- **PostgreSQL 16**

## Arsitektur (layered)
```
Request ─► Router ─► Middleware ─► Controller ─► Service ─► Repository ─► GORM ─► PostgreSQL
```
- **Router** (`internal/router`) — daftar route + wiring dependency (manual DI).
- **Middleware** (`internal/middleware`) — CORS, RequestID, dll.
- **Controller** (`internal/controller`) — parse request, panggil service, tulis response.
- **Service** (`internal/service`) — business logic.
- **Repository** (`internal/repository`) — akses data (GORM).
- **Model** (`internal/model`) — entity/tabel GORM.
- **DTO** (`internal/dto`) — request/response payload + response envelope standar.
- **Config** (`internal/config`) — konfigurasi dari env / `.env`.
- **Database** (`internal/database`) — koneksi & auto-migrate.

## Menjalankan (local)
```bash
cp .env.example .env          # sesuaikan bila perlu
make docker-up                # start PostgreSQL
make install-tools            # sekali saja: install swag CLI
make swag                     # generate docs Swagger
make run                      # jalankan server di :8080
```

- API base path: `http://localhost:8080/api/v1`
- Health check: `GET /api/v1/healthz`
- Swagger UI: `http://localhost:8080/swagger/index.html`

## Perintah Make
Jalankan `make help` untuk daftar lengkap (`run`, `build`, `tidy`, `swag`, `test`, `docker-up`, ...).

## Menambah domain baru
Lihat panduan langkah-demi-langkah di [CLAUDE.md](CLAUDE.md).
