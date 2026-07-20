# ROUTES.md — Katalog Endpoint & Bisnis Logic

> **Living document.** Sumber pengetahuan **bisnis logic** tiap API. Setiap kali menambah endpoint
> baru **atau** mengubah bisnis logic endpoint yang ada, **wajib** update file ini (lihat skill
> `rag-dev` Langkah 5c).
>
> Beda peran dokumen:
> - **Swagger (`docs/`)** = kontrak teknis (schema request/response) — auto-generated.
> - **ROUTES.md (ini)** = *kenapa* & *bagaimana* logika bisnis tiap endpoint bekerja — manual.
> - **`testing/*.md`** = langkah uji + ekspektasi.

Base URL: `http://localhost:8080/api/v1` · Envelope: `internal/response` (`BaseResponse` /
`ErrorResponse`). Semua request dapat `X-Request-ID` untuk tracing (§4b CLAUDE.md).

## Ringkasan endpoint

| Method | Path | Module | Auth | Ringkas |
|---|---|---|---|---|
| GET | `/healthz` | health | – | Cek kesehatan service + DB |
| POST | `/organizations` | organization | **admin** | Buat organization (tenant) |
| GET | `/organizations` | organization | Bearer | List organization |
| GET | `/organizations/{code}` | organization | Bearer | Detail organization |
| PUT | `/organizations/{code}` | organization | **admin** | Update (nama/desc/aktif) |
| DELETE | `/organizations/{code}` | organization | **admin** | Soft delete (guard: tak ada user) |
| POST | `/auth/register` | auth | **admin** | Buat user baru (role `user`, **org divalidasi**) |
| POST | `/auth/register/bulk` | auth | **admin** | Buat banyak user sekaligus (partial success, password auto-generate) |
| POST | `/auth/login` | auth | – | Login → access + refresh JWT |
| POST | `/auth/refresh` | auth | – | Tukar refresh token → token pair baru |
| POST | `/auth/change-password` | auth | Bearer | Ubah password sendiri (revoke sesi lama) |
| POST | `/auth/logout` | auth | Bearer | Revoke refresh token milik sendiri |
| POST | `/auth/forgot-password` | auth | – | Minta link reset password via email (selalu 200) |
| POST | `/auth/reset-password` | auth | – | Set password baru pakai token reset (sekali pakai) |
| GET | `/users/me` | user | Bearer | Profil user dari token |
| GET | `/users/{id}` | user | Bearer | Ambil user (user: se-org · admin: global) |
| PUT | `/users/{id}` | user | Bearer | Update name/password (user: se-org · admin: global) |
| PATCH | `/users/{id}/role` | user | **admin** | Ubah role user (admin/user), admin global |
| DELETE | `/users/{id}` | user | **admin** | Soft delete (admin global) |
| POST | `/uploads/chunk` | upload | Bearer | Upload 1 chunk file besar (resumable, PDF) |
| GET | `/documents` | document | Bearer | List dokumen (user: org sendiri · admin: semua) |
| GET | `/documents/{id}` | document | Bearer | Detail 1 dokumen + presigned URL (tenant-scoped) |
| POST | `/chat/ask` | chat | Bearer | Tanya RAG (buat/lanjut percakapan) |
| GET | `/chat/sessions` | chat | Bearer | List percakapan milik user |
| GET | `/chat/sessions/{id}` | chat | Bearer | Detail percakapan + pesan |
| DELETE | `/chat/sessions/{id}` | chat | Bearer | Hapus percakapan |

> **Auth**: `–` publik · `Bearer` butuh `Authorization: Bearer <access_token>` ·
> **admin** butuh access token dengan role `admin`.

## Rate limiting (429)

Adaptasi konsep elArch (Bucket4j) → **in-memory token bucket** per-kategori (`internal/ratelimit`
+ `middleware.RateLimit`). **Key** = `user:<id>` bila terautentikasi (chain setelah `JWTAuth`),
selain itu `ip:<client-ip>` (cocok untuk endpoint publik/brute-force). Lampaui limit → **`429`**
`{code: RATE_LIMITED}`. Limit per-menit **configurable via env** (`RATELIMIT_*`); default:

| Kategori | Endpoint | Default limit | Key | Alasan |
|---|---|---|---|---|
| `auth` | `POST /auth/login`, `/auth/refresh`, `/auth/forgot-password`, `/auth/reset-password` | 20/menit | **per-IP** | anti brute-force + anti abuse/enumeration reset |
| `chat` | `POST /chat/ask` | 20/menit | per-user | endpoint AI mahal |
| `upload` | `POST /uploads/chunk` | 300/menit | per-user | chunked = banyak request/file (perlu longgar) |

Single-instance (in-memory + janitor evict idle). Multi-instance → nanti pakai Redis (dicatat).

## RBAC (role admin & user)

- **Role** disimpan di `users.role` (default `user`) dan di-embed sebagai claim `role` di JWT.
- **`user`** (default): akses dibatasi organizationCode-nya sendiri (tenant isolation).
- **`admin`** (super-admin, **global**): bypass tenant isolation; satu-satunya yang boleh
  **register** user baru & **soft delete** user, di organization mana pun.
- **Register selalu membuat role `user`.** Admin **tidak** lahir dari API — dibuat/di-promote
  **manual via SQL** (bootstrap):
  ```sql
  -- promote user existing jadi admin
  UPDATE users SET role = 'admin' WHERE email = 'admin@pln.co.id';
  ```
  (Karena register admin-only, admin pertama wajib disiapkan lewat DB langsung.)
- Middleware `RequireRole("admin")` (setelah `JWTAuth`) menjaga endpoint admin → non-admin
  dapat `403 FORBIDDEN_ROLE`, tanpa token `401 UNAUTHORIZED`.

## Konsep JWT & multi-tenant (organizationCode)

- Saat **register**, user menyertakan `organizationCode` (mis. `"pln"`) → disimpan di kolom
  `users.organization_code`.
- Saat **login/refresh**, `organizationCode` + `role` di-embed sebagai **custom claim** di JWT
  (bareng `user_id`, `email`, `token_type`, `exp`). Contoh payload access token:
  `{ "user_id":1, "email":"budi@pln.co.id", "organization_code":"pln", "role":"user", "token_type":"access", ... }`
- Middleware `JWTAuth` mem-parse token tiap request, menaruh `user_id`/`email`/`organization_code`
  ke context. Handler tahu "yang ngehit ini org-nya `pln`" **tanpa query DB**.
- **Dua tipe token**: `access` (pendek, default 15m) untuk akses API; `refresh` (panjang, default
  168h) hanya untuk `/auth/refresh`. Token diberi `token_type` agar refresh tak bisa dipakai
  sebagai access (dan sebaliknya).

---

## Module: health

### GET `/healthz`
- **Tujuan:** liveness/readiness probe. **Auth:** publik. **Request:** –.
- **Bisnis logic:** `repository.Ping` ke Postgres → `ok/up` atau `degraded/down`.
- **Response:** `200` `{status:ok,database:up}` · `503` `{status:degraded,database:down}`.
- **Catatan:** satu-satunya endpoint yang **tidak** memakai `BaseResponse` (shape probe stabil).

---

## Module: auth

### POST `/auth/register`  🔒 admin only
- **Tujuan:** admin membuat akun user baru dalam sebuah organization.
- **Auth:** **admin** (Bearer access token role `admin`). Tanpa token → `401`; role `user` → `403`.
- **Request (`authdto.RegisterRequest`):** `name` (required), `email` (required, email),
  `password` (required, min 6), `organization_code` (required, mis. `"pln"`).
- **Bisnis logic:**
  1. Middleware `JWTAuth` + `RequireRole(admin)` — gerbang sebelum controller.
  2. Bind + validasi (gagal → `400 VALIDATION_ERROR`).
  3. `ExistsByEmail` (scope aktif/non-deleted) — jika ada → `409 EMAIL_TAKEN`.
  4. **Hash password (bcrypt)** sebelum simpan.
  5. `Create` user dengan **role selalu `user`** (`organization_code` dari payload; admin global
     boleh buat user di org mana pun).
  6. Balikan `UserResponse` (memuat `role`) — **password tak pernah** ikut.
- **Aturan data:** email unik **hanya di antara user aktif** (partial unique index
  `idx_users_email_active`) → email user yang sudah soft-deleted boleh dipakai lagi.
- **Response:** `201` (data `UserResponse`) · `400 VALIDATION_ERROR` · `401 UNAUTHORIZED` ·
  `403 FORBIDDEN_ROLE` · `409 EMAIL_TAKEN` · `500 INTERNAL_ERROR`.
- **Logging:** WARN role check gagal · INFO `register: attempt/success` · WARN duplikat.

### POST `/auth/register/bulk`  🔒 admin only
- **Tujuan:** admin membuat **banyak user sekaligus** (onboarding massal). Body = **array of user**.
- **Auth:** **admin** (401 tanpa token · 403 non-admin).
- **Request:** array `BulkRegisterItem` = `[{name, email, organization_code}, ...]`. **Tanpa
  password** (di-generate server). Maks **100 item/request** (`400 BATCH_TOO_LARGE`); array kosong /
  bukan array → `400`.
- **Bisnis logic (model PARTIAL SUCCESS — 1 item gagal TIDAK membatalkan batch):**
  1. `RequireRole(admin)` gate.
  2. Untuk tiap item (berurutan, independen):
     - Validasi `name/email/organization_code` wajib + format email → gagal `VALIDATION_ERROR`.
     - Duplikat email **dalam batch** (case-insensitive) → gagal `DUPLICATE_IN_BATCH`.
     - Email sudah ada di DB (`ExistsByEmail`) → gagal `EMAIL_TAKEN`.
     - **Generate password acak** (crypto/rand, 14 char) → **bcrypt** → `Create` (role selalu
       `user`; org dari item — admin global boleh lintas-org).
  3. Kumpulkan hasil per-item. **Tanpa transaksi** (tiap item mandiri).
- **Response:** selalu `200` bila request diproses. Data `BulkRegisterResponse`:
  `{ total, success_count, failed_count, results:[{index, email, status:"created|failed", id?,
  temp_password?, error_code?, error? }] }`. `temp_password` **hanya** pada item sukses (ditampilkan
  **sekali** agar admin bisa dibagikan).
- **Security:** password plaintext hanya di **response** (never di-log; slog tak pernah memuat
  password). Prod: GORM SQL log mati (level Warn) → hash pun tak ter-log.
- **Logging:** INFO `bulk register: attempt/done` (count/success/failed), INFO per user dibuat,
  ERROR pada kegagalan tak terduga (exists/hash/create).
- **Request (`authdto.LoginRequest`):** `email`, `password`.
- **Bisnis logic:**
  1. `FindByEmail` (scope aktif). 2. `bcrypt.CompareHashAndPassword`.
  3. **Anti user-enumeration:** email tak ada **atau** password salah → error generik yang sama
     (`401 INVALID_CREDENTIALS`). User soft-deleted otomatis tak ditemukan → juga `401`.
  4. Terbitkan **access + refresh** JWT (claim `organization_code` dari user).
- **Response:** `200` (data `TokenResponse{access_token,refresh_token,token_type,expires_in}`) ·
  `400 VALIDATION_ERROR` · `401 INVALID_CREDENTIALS`.
- **Logging:** INFO `login: attempt/success` · WARN invalid credentials.

### POST `/auth/refresh`
- **Tujuan:** perpanjang sesi tanpa login ulang. **Auth:** publik (bawa refresh token di body).
- **Request (`authdto.RefreshRequest`):** `refresh_token`.
- **Bisnis logic:**
  1. Parse & validasi token; **harus `token_type=refresh`** (access token → ditolak).
  2. Pastikan user masih ada (belum soft-deleted) via `FindByID` — jika hilang → `401`.
  3. Terbitkan pasangan token baru.
- **Response:** `200` (data `TokenResponse`) · `400 VALIDATION_ERROR` ·
  `401 INVALID_REFRESH_TOKEN`.
- **Logging:** WARN token invalid · INFO `refresh: success`.
- **Revocation (token_version):** refresh **juga** menolak (`401`) jika `ver` di token ≠
  `users.token_version` — dipakai oleh logout / change-password / reset-password untuk
  membatalkan sesi lama (lihat di bawah).

### Revocation model (token_version) 🔑
Kolom `users.token_version` (default 1) di-embed ke JWT sebagai claim **`ver`**. Setiap
**logout / change-password / reset-password** menaikkan `token_version` → semua refresh token
lama otomatis invalid (cek di `/auth/refresh`). **Trade-off:** access token yang sudah terbit
tetap sah sampai TTL-nya habis (default 15 mnt) — tak ada blacklist per-request (menghindari
dependency Redis). Ini disengaja: revocation efektif di titik refresh, jendela residual ≤ TTL
access.

### POST `/auth/change-password`  🔒 (Bearer)
- **Tujuan:** user mengganti passwordnya sendiri. **Auth:** Bearer access token (user id dari token).
- **Request (`authdto.ChangePasswordRequest`):** `old_password`, `new_password` (min 6).
- **Bisnis logic:**
  1. `FindByID` (dari token) → verifikasi `old_password` via bcrypt; salah → `400
     INVALID_OLD_PASSWORD`.
  2. Hash `new_password` (bcrypt) → `SetPasswordAndBumpVersion` (update password **+**
     `token_version+1` dalam satu statement) → **semua refresh token lama tercabut**.
- **Response:** `200` ("silakan login ulang") · `400 VALIDATION_ERROR`/`INVALID_OLD_PASSWORD` ·
  `401` (tanpa token).
- **Logging:** WARN old-password salah · INFO `change password: success`.

### POST `/auth/logout`  🔒 (Bearer)
- **Tujuan:** cabut sesi (refresh token) milik sendiri. **Auth:** Bearer access token.
- **Request:** tanpa body.
- **Bisnis logic:** `BumpTokenVersion` (user id dari token) → refresh token lama → `401` di
  `/auth/refresh`. Access token tetap sah sampai TTL habis (lihat revocation model).
- **Response:** `200` · `401` (tanpa token).
- **Logging:** INFO `logout: success`.

### POST `/auth/forgot-password`
- **Tujuan:** mulai alur reset password. **Auth:** publik (rate-limited `auth`, per-IP).
- **Request (`authdto.ForgotPasswordRequest`):** `email`.
- **Bisnis logic (anti user-enumeration):**
  1. `FindByEmail`; jika **tidak** ada / error internal → **tetap** balas `200` (tak membocorkan
     keberadaan email).
  2. Jika ada: `InvalidateUserResetTokens` (supersede token lama) → generate token acak
     (`crypto/rand` 32 byte hex) → simpan **hanya SHA-256-nya** (`password_reset_tokens`,
     `expires_at = now + PASSWORD_RESET_TTL`, default 30 mnt) → kirim link
     `{APP_BASE_URL}/reset-password?token=<plain>` via email.
- **Email adapter:** `internal/infra/email` — SMTP asli bila `SMTP_HOST` diset, else **mock**
  (log `WARN "email (MOCK, not sent)"` berisi token) supaya alur dev jalan tanpa SMTP.
- **Response:** **selalu `200`** ("Jika email terdaftar, tautan reset telah dikirim.") ·
  `400 VALIDATION_ERROR` (email tak valid) · `429 RATE_LIMITED`.
- **Logging:** INFO reset token issued / "no such active user (silently ok)".

### POST `/auth/reset-password`
- **Tujuan:** set password baru dengan token dari email. **Auth:** publik (rate-limited `auth`).
- **Request (`authdto.ResetPasswordRequest`):** `token`, `new_password` (min 6).
- **Bisnis logic:**
  1. SHA-256 token → `FindValidResetToken` (`used_at IS NULL AND expires_at > now`); tak ketemu →
     `400 INVALID_RESET_TOKEN` (invalid / kedaluwarsa / sudah dipakai).
  2. Hash `new_password` → `SetPasswordAndBumpVersion` (revoke sesi lama) → `MarkResetTokenUsed`
     (**sekali pakai**).
- **Response:** `200` ("silakan login") · `400 VALIDATION_ERROR`/`INVALID_RESET_TOKEN` ·
  `429 RATE_LIMITED`.
- **Logging:** WARN token invalid/expired · INFO `reset password: success`.

---

## Module: user  🔒 (semua butuh Bearer access token)

**Tenant isolation:** untuk role `user`, operasi hanya boleh menyentuh user dalam
**organizationCode yang sama** → beda org `403 FORBIDDEN_ORGANIZATION`. Role **`admin` bypass**
(global, boleh lintas org). **DELETE khusus admin** (non-admin `403 FORBIDDEN_ROLE`).

### GET `/users/me`
- **Tujuan:** profil user yang sedang login (id diambil dari token).
- **Bisnis logic:** `GetByID(claims.user_id, actorOrg=claims.org)`.
- **Response:** `200` (data `UserResponse`) · `401 UNAUTHORIZED` · `404 USER_NOT_FOUND`.

### GET `/users/{id}`
- **Tujuan:** ambil user by id. **Auth:** Bearer (user: se-org, admin: global).
- **Bisnis logic:** load user; tak ada → `404`; jika non-admin & `org != actorOrg` → `403`.
- **Response:** `200` · `400 VALIDATION_ERROR` (id invalid) · `401` · `403` · `404`.

### PUT `/users/{id}`
- **Tujuan:** update `name` dan/atau `password`. **Email, organizationCode, & role immutable**
  lewat endpoint ini. **Auth:** Bearer (user: se-org, admin: global).
- **Request (`userdto.UpdateUserRequest`):** `name?`, `password?` (hanya field terisi diproses;
  password baru di-**bcrypt**).
- **Bisnis logic:** fetch scoped (admin bypass tenant) → set field → `Save`.
- **Response:** `200` (data `UserResponse`) · `400` · `401` · `403` · `404`.

### PATCH `/users/{id}/role`  🔒 admin only
- **Tujuan:** admin mengubah role user (mis. **promote user → admin**, atau demote admin → user).
  **Auth:** **admin** (global — bisa lintas org).
- **Request (`userdto.UpdateRoleRequest`):** `role` (required, harus `"admin"` atau `"user"`).
- **Bisnis logic:**
  1. `RequireRole(admin)` (non-admin → `403 FORBIDDEN_ROLE`).
  2. Validasi `role` ∈ {admin, user} → selain itu `400 INVALID_ROLE`.
  3. **Self-guard:** admin **tidak boleh** mengubah role dirinya sendiri (`id == actorID`) →
     `400 CANNOT_CHANGE_OWN_ROLE` (cegah self-lockout).
  4. Load user (`404` bila tak ada) → set `role` → `Save`. Idempotent (set role sama → tetap `200`).
- **Response:** `200` (data `UserResponse` dgn `role` baru) · `400 INVALID_ROLE` ·
  `400 CANNOT_CHANGE_OWN_ROLE` · `400 VALIDATION_ERROR` (id/payload) · `401` · `403 FORBIDDEN_ROLE` ·
  `404 USER_NOT_FOUND`.
- **Logging:** WARN invalid role / self-change · INFO `update role: success` (old→new, by_user_id).
- **Bootstrap note:** endpoint ini menggantikan kebutuhan promote via SQL **setelah** ada minimal
  1 admin awal. Admin pertama tetap disiapkan manual via SQL.

### DELETE `/users/{id}`  🔒 admin only
- **Tujuan:** **soft delete** (set `deleted_at`, baris tetap ada). **Auth:** **admin** (global).
- **Bisnis logic:** `RequireRole(admin)` (non-admin → `403 FORBIDDEN_ROLE`) → load user
  (`404` bila tak ada) → `Delete` (GORM soft delete). Admin global: tak ada batas org.
- **Efek:** user tak lagi bisa login/di-query; email-nya bebas dipakai user baru.
- **Response:** `200` (message `"user deleted"`) · `401 UNAUTHORIZED` · `403 FORBIDDEN_ROLE` ·
  `404 USER_NOT_FOUND`.

---

## Module: upload  🔒 (Bearer) — large-file chunked upload

Mekanisme mengikuti elArch: file besar dipecah jadi chunk, tiap chunk di-stream ke **MinIO**,
server menggabung via **compose** (server-side, tanpa download-ulang). "Bulk" = banyak sesi chunk
paralel. Infra: MinIO (`internal/infra/minio`), state sesi in-memory (`sync.Map` + `done` channel).

### POST `/uploads/chunk`
- **Tujuan:** menerima 1 potongan file; menggabung otomatis saat semua chunk lengkap.
- **Auth:** Bearer (semua user terautentikasi, dibatasi kuota). `orgCode` dari token → path objek.
- **Request (multipart/form-data):** `file` (chunk biner), `sessionId`, `fileName` (.pdf),
  `chunkIndex` (0-based), `totalChunks`, `fileSize`, `sha256` (opsional, dedup),
  `chunkSize` (opsional), `forceUpload` (opsional).
- **Bisnis logic:**
  1. **chunk 0 gate:** cek **kuota** (per-role: bulanan+lifetime → `429 QUOTA_EXCEEDED`) →
     **dedup SHA-256** (`409 DUPLICATE_FILE` bila konten identik sudah pernah selesai) →
     validasi ukuran (≤ cap), jumlah chunk, **min chunk 5 MiB** untuk multi-part
     (`400 CHUNK_TOO_SMALL`), nama file (whitelist, anti path-traversal → `400 INVALID_FILENAME`),
     ekstensi `.pdf` + anti double-extension, **MIME sniff** wajib `application/pdf`
     (`400 INVALID_FILE_TYPE`).
  2. Stream chunk → MinIO `temp_chunks/{orgCode}/{sessionId}/{index}`.
  3. Track state sesi (paralel-safe); saat semua chunk hadir → **`ComposeObject`** →
     `uploads/{orgCode}/{sessionId}.pdf`. Koordinasi merge pakai `atomic CAS` + channel `done`
     (non-blocking; perbaikan atas `waitForMerge` elArch yang busy-wait).
  4. Simpan `upload_log` (audit + dedup) + increment kuota; **cleanup** chunk async (goroutine
     + `recover`, delay 5s).
  5. Balikan **presigned URL** (exp konfig, default 3 jam) sebagai preview.
- **Response:** `200` — data `ChunkResult`. Saat belum lengkap: `upload_complete=false`. Saat
  lengkap: `upload_complete=true` + `object_path` + `preview_url`.
  Error: `400` (VALIDATION_ERROR/CHUNK_TOO_SMALL/FILE_TOO_LARGE/INVALID_FILENAME/INVALID_FILE_TYPE) ·
  `401` · `409 DUPLICATE_FILE` · `429 QUOTA_EXCEEDED` · `500 INTERNAL_ERROR`.
- **Aturan penting:** **chunk (non-terakhir) wajib ≥ 5 MiB** (batas S3/MinIO multipart). PDF-only
  (MVP). Ingest RAG (OCR/embed) adalah **langkah terpisah** berikutnya.
- **Logging:** INFO chunk stored / completed · WARN duplikat/kuota/MIME · ERROR compose/store.

## Module: document  🔒 (Bearer) — dokumen hasil upload

Sumber data = `upload_logs` status `completed` (module baca; upload = module tulis). Presigned URL
disertakan agar dokumen bisa **dilihat & diunduh**. **Tenant guard:** user biasa hanya org-nya;
**admin global** (bypass, lihat semua org) — konsisten dgn pola RBAC kita.

### GET `/documents`
- **Tujuan:** daftar dokumen. **Auth:** Bearer.
- **Bisnis logic:** role `user` → filter `organization_code = org(token)`; role `admin` → semua org
  (scope kosong). Urut `created_at DESC`. Tiap item disertai presigned `preview_url`.
- **Response:** `200` (data `[]DocumentResponse` {id, file_name, file_size, total_chunks, sha256,
  organization_code, uploaded_by, object_path, preview_url, created_at}) · `401`.
- **Catatan:** belum ada paginasi (kembalikan semua) — dokumen per-org diasumsikan bounded; tambah
  page/limit bila perlu nanti.

### GET `/documents/{id}`
- **Tujuan:** detail 1 dokumen + URL unduh. **Auth:** Bearer.
- **Bisnis logic:** cari by id (status completed). Tak ada → `404 DOCUMENT_NOT_FOUND`.
  Jika non-admin & `doc.org != org(token)` → `403 FORBIDDEN_ORGANIZATION` (admin bypass).
- **Response:** `200` (data `DocumentResponse` + `preview_url`) · `400 VALIDATION_ERROR` (id invalid)
  · `401` · `403 FORBIDDEN_ORGANIZATION` · `404 DOCUMENT_NOT_FOUND`.
- **Logging:** WARN not-found / cross-org · ERROR tak terduga.

## Module: chat  🔒 (Bearer) — conversation / RAG Q&A (Core #3)

Percakapan tanya-jawab terhadap dokumen. Backend = **orchestrator**: menyimpan sesi+pesan, lalu
meneruskan pertanyaan ke **AI client** (`internal/infra/ai`). Saat ini AI-nya **mock** — tinggal
ganti ke HTTP client tim AI saat kontrak siap (§8c). Model: **2 tabel** `chat_sessions` +
`chat_messages` (1 pesan = 1 row, role `user`/`assistant`).

**Konsep session:** `session_id` = **UUID dari client** (id sama = percakapan sama; id baru =
percakapan baru). Tak ada endpoint "create session" — dibuat otomatis saat `ask` pertama.
**Sliding window 20 sesi/user** (tertua di-evict). **Scope per-user** (hanya lihat percakapan
sendiri); `organization_code` dari token diteruskan ke AI sebagai **filter retrieval** (tenant).

### POST `/chat/ask`
- **Tujuan:** kirim pertanyaan; buat/lanjut percakapan. **Auth:** Bearer.
- **Request:** `{ session_id (required, UUID-like), question (required) }`.
- **Bisnis logic:**
  1. Validasi `session_id` (UUID-like) → `400 INVALID_SESSION`.
  2. Load session. **Milik user lain → `404 SESSION_NOT_FOUND`** (tak bisa reuse UUID orang;
     tak bocorkan). Belum ada → buat baru (title = 80 char pertama pertanyaan; enforce sliding
     window 20).
  3. Simpan pesan `user` → panggil **AI** (`question, organization_code, thread_id=session_id`) →
     simpan pesan `assistant`. **AI gagal → jawaban fallback ramah (tetap `200`)**, di-log.
  4. `updated_at` sesi di-touch (best-effort).
- **Response:** `200` (data `{session_id, answer}`) · `400 VALIDATION_ERROR`/`INVALID_SESSION` ·
  `401` · `404 SESSION_NOT_FOUND`.
- **Logging:** INFO new session / ask / answered · WARN invalid/ownership · ERROR AI/DB.

### GET `/chat/sessions`
- List sesi milik user, urut `updated_at DESC`. Response `[]{id,title,created_at,updated_at}`.

### GET `/chat/sessions/{id}`
- Detail 1 sesi + semua pesan (urut `created_at ASC`). **Hanya pemilik** (else `404`).
- Response `{id, title, organization_code, messages:[{id,role,content,created_at}]}`.

### DELETE `/chat/sessions/{id}`
- Hapus sesi + semua pesannya (transaksi). **Hanya pemilik** (else `404`). Response `200`.

## Module: organization  🔒 (Bearer) — registry tenant

**Fondasi multi-tenant.** `organization_code` di seluruh app (user, document, chat, retrieval AI)
kini **wajib merujuk** `organizations.code` yang **ada & aktif** — bukan string bebas. Kode
di-normalisasi **trim whitespace** (`"pln "` == `"pln"`). Existing org (`pln`,`icon`) di-seed dari
users saat migrate. Read (list/get) = semua user login; write = **admin**.

### POST `/organizations`  🔒 admin
- **Request:** `{code (required, 2-64 alnum/-/_), name (required), description?}`.
- **Bisnis logic:** validasi format code (`400 INVALID_CODE`) → cek unik (`409 ORG_EXISTS`) → create (active=true).
- **Response:** `201` · `400 INVALID_CODE` · `401` · `403` · `409 ORG_EXISTS`.

### GET `/organizations` · GET `/organizations/{code}`
- List semua / detail by code. `404 ORG_NOT_FOUND` bila tak ada.

### PUT `/organizations/{code}`  🔒 admin
- Update `name`/`description`/`active` (pointer → field diabaikan bila null). **Deactivate**
  (`active:false`) memblokir registrasi baru ke org itu, **tapi user existing tetap jalan**.
- **Response:** `200` · `400` · `401` · `403` · `404`.

### DELETE `/organizations/{code}`  🔒 admin
- **Soft delete**, dengan **guard**: ditolak `409 ORG_HAS_USERS` bila masih ada user aktif.
- **Response:** `200` · `401` · `403` · `404` · `409 ORG_HAS_USERS`.

### Dampak ke auth (validasi org)
`POST /auth/register` & `/auth/register/bulk` kini **memvalidasi** `organization_code`
(`ExistsActive`): org tak dikenal/nonaktif → register `400 INVALID_ORGANIZATION`; bulk → item
`failed` code `INVALID_ORGANIZATION` (partial success). Org code disimpan ter-trim.

## Konvensi menulis entri baru

Dokumentasikan minimal: **Tujuan · Auth · Request (+validasi) · Bisnis logic (langkah +
aturan/edge case) · Response (sukses + tiap error code) · Logging**. Tambahkan baris di tabel
"Ringkasan endpoint". Fokus pada *keputusan bisnis* — bukan schema (itu Swagger).
