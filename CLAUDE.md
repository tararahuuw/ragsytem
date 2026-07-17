# CLAUDE.md — RAG System Backend

> Living document. **Selalu perbarui saat menambah domain, endpoint, atau keputusan arsitektur.**
> Tujuan: siapa pun (atau Claude) yang membuka repo ini langsung paham pola & cara menambah fitur
> tanpa menebak.

---

## 1. Ringkasan

Backend API untuk aplikasi **RAG (NotebookLM-like)**: user upload dokumen → di-ingest →
tanya-jawab terhadap isi dokumen. Saat ini masih tahap **scaffold**: baru ada 1 vertical slice
(`healthcheck`) sebagai contoh pola. Fitur RAG (upload, ingest, embedding, chat) menyusul.

**Stack:** Go 1.26 · Gin · GORM (PostgreSQL) · swaggo/swag (Swagger) · Docker.
**Module path:** `github.com/tararahuuw/ragsytem`

---

## 2. Arsitektur berlapis

Aliran request selalu satu arah:

```
Router → Middleware → Controller → Service → Repository → GORM → PostgreSQL
```

Aturan ketergantungan (dependency rule): lapisan atas boleh memanggil lapisan di bawahnya,
**tidak sebaliknya**. Controller tidak boleh akses GORM langsung; Service tidak tahu soal HTTP.

| Lapisan | Package | Tanggung jawab | Tidak boleh |
|---|---|---|---|
| Router | `internal/router` | Definisi route + wiring dependency (manual DI) | logic bisnis |
| Middleware | `internal/middleware` | Cross-cutting: CORS, RequestID, auth, dll | logic domain |
| Controller | `internal/controller` | Bind/validasi request, panggil service, tulis JSON | akses DB langsung |
| Service | `internal/service` | Business logic, orkestrasi | tahu soal `gin.Context`/HTTP |
| Repository | `internal/repository` | Akses data via GORM | logic bisnis |
| Model | `internal/model` | Entity/tabel GORM | — |
| DTO | `internal/dto` | Payload request/response + envelope standar | — |
| Config | `internal/config` | Baca env / `.env` | — |
| Database | `internal/database` | Koneksi pool + `AutoMigrate` | — |

**Interface di mana?** Repository & Service dideklarasikan sebagai **interface** (lalu ada
implementasi `struct` privat + constructor `New...`). Ini memudahkan mock saat testing dan
menjaga batas antar lapisan. Controller memegang `*struct` konkret (tidak perlu interface).

---

## 3. Struktur folder

```
ragsytem/
├── main.go                    # entry point + anotasi @title Swagger global
├── internal/
│   ├── config/                # Config struct + Load() dari env
│   ├── database/              # koneksi GORM + Migrate()
│   ├── logger/                # setup slog + helper request-scoped (FromContext)
│   ├── response/              # BaseResponse, ErrorResponse + helper (Success/Error/...)
│   ├── jwt/                   # generate/parse JWT (claims: user_id, email, organization_code, role, token_type)
│   ├── rbac/                  # konstanta role (RoleAdmin, RoleUser)
│   ├── infra/minio/           # wrapper MinIO (Put, Compose, List, Remove, Exists, Presign)
│   ├── middleware/            # cors, request_id, access_log, recovery, auth (JWTAuth, RequireRole, CurrentOrgCode/Role/UserID)
│   ├── router/                # router.go (aggregator) + <module>/route.go
│   │   ├── health/route.go    #   Register(v1, db)
│   │   ├── auth/route.go      #   Register(v1, cfg, db) — public
│   │   └── user/route.go      #   Register(v1, cfg, db) — protected (JWTAuth)
│   ├── controller/<module>/   # package <module> → Controller, NewController
│   │   ├── health/  · auth/  · user/
│   ├── service/<module>/      # package <module> → Service, NewService, ErrXxx
│   │   ├── health/  · auth/  · user/
│   ├── repository/<module>/   # package <module> → Repository, NewRepository(db)
│   │   ├── health/  · user/   #   auth pakai repository/user (GORM, soft delete)
│   ├── model/<module>/        # entity per-module: model/user/, model/upload/
│   └── dto/<module>/          # DTO per-module: dto/auth/, dto/user/, dto/health/, dto/upload/
├── testing/                   # playbook test API (.md) per rumpun endpoint
├── postman/                   # collection + environment (import-ready) — sinkron dgn API
├── ROUTES.md                  # katalog bisnis logic per-endpoint (living doc)
├── docs/                      # GENERATED oleh swag (jangan edit manual)
├── db/migrations/             # (opsional) migrasi SQL manual bila diperlukan
├── docker-compose.yaml        # PostgreSQL lokal
├── Dockerfile                 # multi-stage build
├── Makefile                   # run/build/tidy/swag/test/docker
├── .env.example               # contoh konfigurasi
└── README.md
```

---

## 4. Konvensi

- **Foldering per-module:** tiap layer (`router`, `controller`, `service`, `repository`) punya
  **subfolder per module**: `internal/<layer>/<module>/`. Contoh: `internal/service/auth/`.
  Package = nama module (mis. `package auth`), sehingga tipe di dalamnya idiomatis:
  `Repository`, `Service`, `Controller` + constructor `NewRepository/NewService/NewController`
  (bukan `AuthService` — hindari stutter `auth.AuthService`).
- **Import antar-layer pakai alias** biar jelas & tak bentrok (nama package sama antar layer):
  `authsvc "…/internal/service/auth"`, `authrepo "…/internal/repository/auth"`, dst.
- **DTO & model juga per-module** di dalam foldernya: `internal/dto/<module>/` (package `<module>`,
  file `<module>_dto.go`) dan `internal/model/<module>/`. Import pakai alias, mis.
  `authdto "…/internal/dto/auth"`, `authmodel "…/internal/model/auth"`.
- **Route self-contained:** tiap `internal/router/<module>/route.go` punya `Register(rg, db)`
  yang melakukan wiring `repo→service→controller` + mount route module-nya sendiri.
  `internal/router/router.go` hanya memanggil `Register` tiap module.
- **Response envelope (WAJIB):** semua endpoint umum pakai package `internal/response`:
  - Sukses → `response.OK(c, msg, data)` / `response.Created(c, msg, data)` (bentuk `BaseResponse`
    `{success, message, data}`).
  - Error → `response.Error(c, status, msg, "CODE")` (bentuk `ErrorResponse`
    `{success:false, message, error:{code}}`); validasi → `response.ValidationError(c, msg, details)`.
  - Pengecualian: `health` mengembalikan struct probe langsung (bukan envelope) agar shape stabil.
- **Error:** service kembalikan `error` (sentinel `ErrXxx`); controller petakan via `errors.Is`
  ke status + `response.Error` dengan `code` machine-readable (mis. `EMAIL_TAKEN`).
- **Context:** teruskan `ctx.Request.Context()` dari controller → service → repository (membawa
  `request_id` untuk logging).
- **Swagger:** setiap handler diberi anotasi `// @Summary ... @Router ...`. Regenerate dengan
  `make swag` setiap kali anotasi/route berubah. Base path global = `/api/v1`.
- **Migrasi:** default pakai `AutoMigrate` (daftarkan model di `database.Migrate`). Untuk perubahan
  yang butuh kontrol (index, data migration), taruh SQL di `db/migrations/`.

---

## 4b. Logging (WAJIB untuk setiap API) 🔑

> Highlight tim (VP IT): **setiap pembuatan/perubahan API HARUS disertai log yang mudah dibaca
> dan lengkap** — mencakup jalur INFO (alur normal) maupun ERROR/WARN — supaya bisa di-**trace**
> end-to-end. Jangan pernah menelan error tanpa log.

**Tooling:** `log/slog` (structured). Setup di `internal/logger`:
- `logger.Init(env)` — text readable saat dev, **JSON** saat production. Dipanggil di `main.go`.
- `logger.FromContext(ctx)` — mengembalikan `*slog.Logger` yang sudah ter-tag `request_id`,
  sehingga log controller → service → repository untuk satu request bisa dikorelasikan.

**Otomatis:** middleware `AccessLog` menulis 1 baris `http_request` per request
(`method, path, status, latency_ms, client_ip, request_id`); level: `5xx`→Error, `4xx`→Warn,
selebihnya Info. `request_id` di-generate/di-reuse oleh middleware `RequestID` dan disuntik ke
`context` + response header `X-Request-ID`.

**Wajib ditulis manual di service/handler:**
- **INFO** di titik penting alur bisnis: attempt & success. Contoh:
  `log.Info("register: attempt", "email", email)` lalu `log.Info("register: success", "user_id", id)`.
- **WARN** untuk kegagalan yang diharapkan (validasi, kredensial salah, konflik):
  `log.Warn("login: rejected, invalid credentials", "email", email)`.
- **ERROR** untuk kegagalan tak terduga (DB error, dependency down), sertakan `"error", err`:
  `log.Error("register: failed to create user", "email", email, "error", err)`.

**Aturan:**
- Ambil logger via `logger.FromContext(ctx)` — **selalu bawa `request_id`**.
- Pakai **key–value terstruktur**, bukan `fmt.Sprintf` di message. Message singkat & konsisten
  (`"<area>: <peristiwa>"`), detail di attribute.
- **JANGAN log data sensitif** (password, token, PII penuh). Email boleh untuk tracing; jangan
  log body mentah yang memuat kredensial.
- Setiap cabang `error` di service/controller = minimal satu log (Warn/Error) sebelum return.

Contoh nyata (satu request punya `request_id` sama di semua baris):
```
level=INFO msg="register: attempt"  request_id=544e… email=sari@example.com
level=INFO msg="register: success"  request_id=544e… user_id=1 email=sari@example.com
level=INFO msg=http_request method=POST path=/api/v1/auth/register status=201 latency_ms=0 request_id=544e…
```

---

## 4c. Error handling & panic recovery ("try/catch" ala Go) 🔑

> Highlight tim (VP IT): **kode harus kuat menangani kegagalan** — tidak boleh ada error yang
> ditelan diam-diam, dan panic tak terduga tidak boleh membuat server crash. Go tidak punya
> `try/catch`; padanannya = **error return yang disiplin** + **`defer`/`recover`** sebagai jaring
> pengaman.

**1. Error sebagai nilai (jalur normal).**
- Setiap fungsi yang bisa gagal mengembalikan `error`. **Selalu cek `if err != nil`** — jangan
  pernah abaikan (`_ = err`) kecuali benar-benar tidak relevan dan diberi komentar.
- Service memakai **sentinel error** (`var ErrXxx = errors.New(...)`); controller memetakannya
  via `errors.Is` ke HTTP status + `response.Error(c, status, msg, "CODE")`.
- Bungkus error saat menaikkan konteks: `fmt.Errorf("create user: %w", err)` (pakai `%w` agar
  `errors.Is/As` tetap jalan).
- Setiap cabang error **wajib di-log** (Warn/Error) sebelum return — lihat §4b.

**2. Panic recovery (jaring pengaman global).**
- Middleware `middleware.Recovery()` (paling luar setelah `RequestID`) memakai `defer`/`recover`
  untuk menangkap **semua** panic di lifecycle request. Ia: mlog `Error` lengkap (`panic`,
  `method`, `path`, `stack`, `request_id`) + balas **`response.Error` 500 `INTERNAL_ERROR`** yang
  standar. Server tetap hidup; stack trace **tidak** bocor ke client.
- Ini menggantikan `gin.Recovery()` default (yang balas 500 polos tanpa envelope & tanpa slog).

**3. `defer` untuk cleanup.**
- Pakai `defer` untuk melepas resource (row.Close(), unlock, dsb) agar tetap jalan walau ada
  error/panic di tengah.

**4. Goroutine.**
- Panic di goroutine **tidak** tertangkap oleh middleware Recovery. Jika membuat goroutine
  (worker, async task), **wajib** pasang `defer func(){ if r:=recover(); r!=nil { log... } }()`
  di dalam goroutine itu sendiri.

**Aturan singkat:** tiap kemungkinan gagal → cek error + log + response envelope. Tiap operasi
yang bisa panic tak terduga → tertutup Recovery (request) atau recover lokal (goroutine).
Jangan pernah `panic()` untuk alur bisnis normal — pakai `error`.

---

## 5. Cara menambah domain baru (resep)

Misal menambah domain `document` (ikuti pola module `auth`):

1. **Model** — `internal/model/document/document.go` (package `document`): entity GORM `Document`.
2. **Daftarkan migrasi** — `internal/database/database.go` → `db.AutoMigrate(&documentmodel.Document{})`.
3. **DTO** — `internal/dto/document/document_dto.go` (package `document`): request & response payload.
4. **Repository** — `internal/repository/document/document_repository.go` (package `document`):
   `type Repository interface` + `NewRepository(db)`.
5. **Service** — `internal/service/document/document_service.go` (package `document`):
   `type Service interface` + `NewService(repo)`, sentinel `ErrXxx` bila perlu.
   **Wajib pasang log** (INFO attempt/success, WARN/ERROR di tiap cabang gagal) via
   `logger.FromContext(ctx)` — lihat §4b.
6. **Controller** — `internal/controller/document/document_controller.go` (package `document`):
   `type Controller` + `NewController(svc)` + handler. Pakai `response.OK/Created/Error/
   ValidationError` (bukan `c.JSON` mentah) + anotasi Swagger (`response.BaseResponse{data=...}`).
7. **Route + DI** — `internal/router/document/route.go` (package `document`): `Register(rg, db)`
   wiring repo→service→controller + mount route. Lalu daftarkan di `internal/router/router.go`:
   `documentroute.Register(v1, db)` (import alias `documentroute`).
8. **Swagger** — `make swag`. **Docs bisnis** — update `ROUTES.md` (bisnis logic per-endpoint).
   **Postman** — update `postman/*.json` (collection + environment). **Testing** — buat
   `testing/document_test.md`. Lalu `make run`.

Ikuti module `auth` (register/login) sebagai contoh lengkap end-to-end: foldering per-module,
response envelope, dan logging tracing.

> **`ROUTES.md`** = katalog pengetahuan **bisnis logic** tiap endpoint (kenapa & bagaimana).
> Wajib diperbarui setiap ada endpoint baru atau perubahan bisnis logic (skill `rag-dev` Langkah 5c).
> Swagger = kontrak schema (auto); ROUTES.md = alasan bisnis (manual); `testing/` = uji.

---

## 5b. Testing API (folder `testing/`)

Setiap rumpun endpoint punya **playbook** Markdown di `testing/` (mis. `authentication_test.md`)
berisi langkah konkret + ekspektasi yang bisa Claude eksekusi lalu lapor PASS/FAIL. Fokus:
smoke test (deteksi endpoint mati / `5xx` / kontrak berubah).

- Konvensi & format laporan: `testing/README.md`. Template case baru: `testing/_TEMPLATE.md`.
- **Saat menambah domain baru, buat juga `testing/<domain>_test.md`** dan daftarkan di
  `testing/README.md`.
- Menjalankan: user bilang `test module <nama>` atau `test semua module di testing`.

## 5c. Skill pengembangan (`/rag-dev`)

Ada Claude skill di `.claude/skills/rag-dev/` yang menuntun **pengembangan fitur end-to-end**
(11 langkah): pahami maksud → baca CLAUDE.md → telusuri file lintas-layer → inspeksi tabel DB
(read-only, **lokal/dev saja, dilarang production**) → implementasi → smoke test API → update
`testing/` → smoke test 1 module → review security & business logic → update CLAUDE.md → summary.
Summon dengan `/rag-dev <perintah>`. Kalau maksud command belum jelas, skill akan bertanya dulu.

## 6. Konfigurasi (env)

Semua via environment variable (lihat `.env.example`). Default aman untuk local.

| Var | Default | Keterangan |
|---|---|---|
| `APP_NAME` | ragsystem | nama app |
| `APP_ENV` | development | `development`/`staging`/`production` (prod → gin ReleaseMode + log Warn) |
| `SERVER_HOST` / `SERVER_PORT` | 0.0.0.0 / 8080 | bind server |
| `DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASSWORD`/`DB_NAME`/`DB_SSLMODE` | localhost/5432/postgres/postgres/ragsystem/disable | PostgreSQL |
| `JWT_SECRET` | change-me-in-production | kunci tanda tangan JWT (HS256) |
| `JWT_ACCESS_TTL` / `JWT_REFRESH_TTL` | 15m / 168h | umur access & refresh token (durasi Go atau detik) |
| `MINIO_ENDPOINT`/`MINIO_ACCESS_KEY`/`MINIO_SECRET_KEY`/`MINIO_BUCKET`/`MINIO_USE_SSL` | localhost:9000/minioadmin/minioadmin/ragsystem/false | Object storage (upload) |
| `UPLOAD_MAX_FILE_SIZE` / `UPLOAD_PREVIEW_EXPIRY` | 524288000 (500MB) / 3h | cap ukuran file & umur presigned URL |

---

## 7. Menjalankan & tooling

```bash
make docker-up        # PostgreSQL lokal
make install-tools    # sekali: install swag CLI (github.com/swaggo/swag/cmd/swag)
make swag             # generate ./docs
make run              # server :8080
```
- Health: `GET /api/v1/healthz`
- Swagger UI: `http://localhost:8080/swagger/index.html`

`swag` CLI ter-install di `$(go env GOPATH)/bin` — pastikan ada di `PATH`.

---

## 8. Status & rencana

- [x] Scaffold layered (Gin + GORM), health slice, Swagger, Docker, Makefile.
- [x] Foldering per-module di tiap layer (router/controller/service/repository/model/dto/<module>/).
- [x] **Auth real**: register/login/refresh, GORM `users` (soft delete), bcrypt, **JWT
      access+refresh** dengan claim `organization_code`, middleware `JWTAuth`.
- [x] **User management**: `/users/me`, GET/PUT/DELETE `/users/{id}` (soft delete),
      **isolasi tenant** per organizationCode.
- [x] **RBAC (admin/user)**: role di JWT + `RequireRole`. Register & soft-delete **admin-only**;
      admin **global** (bypass tenant). Bootstrap admin manual via SQL.
- [x] **Ubah role via API**: `PATCH /users/{id}/role` (admin-only, admin/user, self-guard).
- [x] **Upload file besar (chunked)** ala elArch: `POST /uploads/chunk` → MinIO → compose →
      presigned; validasi (PDF/MIME/nama), dedup SHA-256, kuota per-role, cleanup async.
- [ ] Auth lanjutan: forgot/reset password, revoke refresh token.
- [ ] Ingestion RAG (ekstraksi teks / chunking / embedding) dari objek hasil upload.
- [ ] Integrasi mesin embedding + vector store.
- [ ] Endpoint chat / Q&A (RAG) + riwayat percakapan.

> Referensi desain sistem serupa yang sudah jadi: lihat `../elarch/CLAUDE.md` (pola upload
> chunked, proxy LLM/OCR, access-control retrieval, chat history).

---

## 9. Changelog keputusan (append di sini)

- **2026-07-16** — Inisiasi backend. Pilihan stack: Gin + GORM + swaggo. Manual DI di router
  (belum perlu wire/fx untuk skala kecil-menengah). Response envelope `dto.APIResponse`.
- **2026-07-16** — Refactor ke **foldering per-module** di tiap layer
  (`internal/<layer>/<module>/`). Tipe di-rename idiomatis (`Repository/Service/Controller`).
  Tiap module punya `router/<module>/route.go` dengan `Register(rg, db)` yang wiring sendiri;
  `router.go` jadi aggregator. Tambah module **auth dummy** (register/login, repo in-memory,
  password belum di-hash, token dummy) sebagai contoh module ke-2 selain health.
- **2026-07-16** — **model & dto** ikut per-module (`internal/model/<module>/`,
  `internal/dto/<module>/`). Tambah package **`internal/response`** (`BaseResponse`/`ErrorResponse`
  + helper) sebagai standar envelope. Tambah **structured logging** `log/slog`
  (`internal/logger`) + middleware `AccessLog` + propagasi `request_id` lewat context →
  **logging jadi standar wajib tiap API** (lihat §4b). Swagger di-generate dengan
  `--parseDependency --parseInternal` (tipe lintas-package).
- **2026-07-16** — **Error handling & panic recovery** dijadikan standar (§4c). Ganti
  `gin.Recovery()` default dengan `middleware.Recovery()` custom: recover panic → log Error
  (stack + request_id) → balas `response.Error` 500 standar (tak bocor stack ke client). Tambah
  `recovery_test.go`. Disiplin error-as-value + wrap `%w` + wajib log tiap cabang error.
- **2026-07-16** — Tambah `ROUTES.md` (katalog bisnis logic per-endpoint) + folder `postman/`
  (collection v2.1.0 + environment lokal, login auto-capture token). Skill `rag-dev` dapat
  Langkah 5c (update ROUTES.md) & 5d (update Postman) untuk API baru / perubahan bisnis logic.
- **2026-07-16** — **Auth + User (real, via /rag-dev)**. Ganti auth dummy in-memory →
  persistensi GORM. Module baru: `user` (model/repository/service/controller/router) + `auth`
  di-refactor pakai `repository/user`. Tambah `internal/jwt` (HS256, claim `organization_code`
  + `token_type` access/refresh), `middleware.JWTAuth` + helper `CurrentOrgCode/UserID`.
  Password **bcrypt**. **Soft delete** via `gorm.DeletedAt` + partial unique index
  `idx_users_email_active` (email unik hanya antar user aktif → boleh reuse setelah delete).
  Endpoint: `POST /auth/{register,login,refresh}`, `GET /users/me`, `GET|PUT|DELETE /users/{id}`.
  **Isolasi tenant**: operasi user dibatasi `organizationCode` token (beda org → 403). Config
  JWT (`JWT_SECRET`, `JWT_ACCESS_TTL`, `JWT_REFRESH_TTL`). Unit test: `internal/jwt`,
  `middleware` (JWTAuth), router (protected routes butuh token).
- **2026-07-17** — **RBAC admin/user (via /rag-dev)**. Tambah `internal/rbac` (RoleAdmin/RoleUser),
  claim `role` di JWT, `middleware.RequireRole` + `CurrentRole`. Kolom `users.role`
  (default `user`, backfill di migrate). **Register & DELETE user jadi admin-only**
  (`JWTAuth`+`RequireRole(admin)`); register selalu bikin role `user`. **Admin = super-admin
  global** (bypass isolasi tenant lintas org); role `user` tetap tenant-scoped. Bootstrap admin
  **manual via SQL** (`UPDATE users SET role='admin' WHERE email=...`) — tak ada auto-seed.
  Unit test: `RequireRole`, router RBAC gating (register/delete: 401 no-token, 403 non-admin).
- **2026-07-17** — **Upload file besar (chunked) ala elArch (via /rag-dev)**. Infra baru
  **MinIO** (`internal/infra/minio`, SDK `minio-go`). Module `upload`
  (controller/service/repository/dto/model/router). Endpoint `POST /uploads/chunk` (JWT):
  stream chunk → MinIO `temp_chunks/{org}/{sid}/{i}`, **compose** server-side →
  `uploads/{org}/{sid}.pdf`, balikan **presigned URL**. Validasi: PDF-only + **MIME sniff**
  (`mimetype`), whitelist nama (anti path-traversal/double-ext), cap 500MB, **min chunk 5 MiB**
  (batas S3 compose). **Dedup SHA-256** (`upload_logs`), **kuota per-role** bulanan+lifetime
  (`upload_quota_configs`/`usages`, di-seed migrate). Koordinasi merge **non-blocking** (atomic
  CAS + channel `done`) — perbaikan atas `waitForMerge` elArch. Cleanup chunk async (goroutine +
  recover). Smoke test terbukti **byte-perfect** (download presigned == sha256 asli). Unit test
  validasi nama/ekstensi. Config MinIO + upload di `config`/`.env`; docker-compose + MinIO.
- **2026-07-17** — Endpoint **ubah role** `PATCH /users/{id}/role` (admin-only via
  `RequireRole(admin)`): set role `admin`/`user` (validasi `rbac.IsValidRole` → 400
  `INVALID_ROLE`), admin global (tanpa tenant check), **self-guard** (tak bisa ubah role sendiri →
  400 `CANNOT_CHANGE_OWN_ROLE`), 404 bila user tak ada. Reuse `repository/user.Update`. Ini
  melengkapi bootstrap SQL → promote/demote kini bisa lewat API (setelah admin pertama ada).
