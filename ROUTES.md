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
| POST | `/auth/register` | auth | **admin** | Buat user baru (role selalu `user`), bcrypt |
| POST | `/auth/login` | auth | – | Login → access + refresh JWT |
| POST | `/auth/refresh` | auth | – | Tukar refresh token → token pair baru |
| GET | `/users/me` | user | Bearer | Profil user dari token |
| GET | `/users/{id}` | user | Bearer | Ambil user (user: se-org · admin: global) |
| PUT | `/users/{id}` | user | Bearer | Update name/password (user: se-org · admin: global) |
| DELETE | `/users/{id}` | user | **admin** | Soft delete (admin global) |

> **Auth**: `–` publik · `Bearer` butuh `Authorization: Bearer <access_token>` ·
> **admin** butuh access token dengan role `admin`.

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

### POST `/auth/login`
- **Tujuan:** autentikasi → terbitkan token. **Auth:** publik.
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

### DELETE `/users/{id}`  🔒 admin only
- **Tujuan:** **soft delete** (set `deleted_at`, baris tetap ada). **Auth:** **admin** (global).
- **Bisnis logic:** `RequireRole(admin)` (non-admin → `403 FORBIDDEN_ROLE`) → load user
  (`404` bila tak ada) → `Delete` (GORM soft delete). Admin global: tak ada batas org.
- **Efek:** user tak lagi bisa login/di-query; email-nya bebas dipakai user baru.
- **Response:** `200` (message `"user deleted"`) · `401 UNAUTHORIZED` · `403 FORBIDDEN_ROLE` ·
  `404 USER_NOT_FOUND`.

---

## Konvensi menulis entri baru

Dokumentasikan minimal: **Tujuan · Auth · Request (+validasi) · Bisnis logic (langkah +
aturan/edge case) · Response (sukses + tiap error code) · Logging**. Tambahkan baris di tabel
"Ringkasan endpoint". Fokus pada *keputusan bisnis* — bukan schema (itu Swagger).
