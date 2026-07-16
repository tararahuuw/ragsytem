# ROUTES.md — Katalog Endpoint & Bisnis Logic

> **Living document.** Sumber pengetahuan **bisnis logic** tiap API. Setiap kali menambah endpoint
> baru **atau** mengubah bisnis logic endpoint yang ada, **wajib** update file ini (lihat skill
> `rag-dev` Langkah 5c).
>
> Beda peran dokumen:
> - **Swagger (`docs/`)** = kontrak teknis (schema request/response) — auto-generated.
> - **ROUTES.md (ini)** = *kenapa* & *bagaimana* logika bisnis tiap endpoint bekerja — ditulis manual.
> - **`testing/*.md`** = langkah uji + ekspektasi.

Base URL: `http://localhost:8080/api/v1` · Envelope: lihat `internal/response`
(`BaseResponse` / `ErrorResponse`). Semua request dapat `X-Request-ID` untuk tracing (§4b CLAUDE.md).

## Ringkasan endpoint

| Method | Path | Module | Auth | Ringkas |
|---|---|---|---|---|
| GET | `/healthz` | health | – | Cek kesehatan service + DB |
| POST | `/auth/register` | auth | – | Daftar user baru (dummy, in-memory) |
| POST | `/auth/login` | auth | – | Login, kembalikan token dummy |

> Kolom **Auth**: `–` = publik; nanti isi `Bearer` untuk endpoint terproteksi.

---

## Module: health

### GET `/healthz`
- **Tujuan:** liveness/readiness probe untuk uptime monitor & orchestrator.
- **Auth:** publik.
- **Request:** tidak ada.
- **Bisnis logic:**
  1. Service memanggil `repository.Ping(ctx)` → `sqlDB.PingContext` ke PostgreSQL.
  2. Ping sukses → `status=ok`, `database=up`. Ping gagal → `status=degraded`, `database=down`.
- **Response (bukan envelope — sengaja shape probe stabil):**
  - `200 OK` → `{"status":"ok","database":"up"}`
  - `503 Service Unavailable` → `{"status":"degraded","database":"down"}`
- **Catatan:** endpoint ini satu-satunya yang **tidak** memakai `BaseResponse`, agar mudah diparse
  probe eksternal. Server hanya bisa start jika DB terkoneksi (lihat `main.go`).

---

## Module: auth  ⚠️ DUMMY

> Status: **dummy** — user disimpan **in-memory** (hilang saat restart), password **belum di-hash**,
> token masih `dummy-token-...` (bukan JWT). Rencana upgrade: persistensi GORM (`users`), bcrypt,
> JWT, middleware otorisasi, `/auth/me`, forgot/reset password.

### POST `/auth/register`
- **Tujuan:** membuat akun user baru.
- **Auth:** publik.
- **Request (`authdto.RegisterRequest`):**
  | Field | Rule | Catatan |
  |---|---|---|
  | `name` | required | |
  | `email` | required, format email | jadi identitas unik |
  | `password` | required, min 6 | |
- **Bisnis logic:**
  1. Bind + validasi payload (gagal → `400 VALIDATION_ERROR`, detail per-field).
  2. `repo.ExistsByEmail(email)` — jika sudah ada → tolak (`ErrEmailTaken` → `409`).
     Email = **kunci unik**; tidak boleh duplikat.
  3. `repo.Create(user)` — assign `id` (sequence) + `created_at`.
     ⚠️ DUMMY: password disimpan apa adanya. Versi real **wajib** bcrypt sebelum simpan.
  4. Kembalikan `UserResponse` — **password tidak pernah** ikut (field `json:"-"`).
- **Response:**
  - `201 Created` → `BaseResponse{ data: UserResponse{id,name,email} }`, message `"user registered"`.
  - `400` code `VALIDATION_ERROR` — payload invalid.
  - `409` code `EMAIL_TAKEN` — email sudah terdaftar.
  - `500` code `INTERNAL_ERROR` — kegagalan tak terduga.
- **Logging:** INFO `register: attempt` → INFO `register: success` (user_id) | WARN saat email
  duplikat / payload invalid | ERROR saat gagal create.

### POST `/auth/login`
- **Tujuan:** autentikasi kredensial, kembalikan token.
- **Auth:** publik.
- **Request (`authdto.LoginRequest`):** `email` (required, email), `password` (required).
- **Bisnis logic:**
  1. Bind + validasi (gagal → `400 VALIDATION_ERROR`).
  2. `repo.FindByEmail(email)`.
  3. **Anti user-enumeration:** jika user tidak ditemukan **atau** password tidak cocok →
     error **generik** yang sama (`ErrInvalidCredentials` → `401 INVALID_CREDENTIALS`). Tidak
     membedakan "email tak ada" vs "password salah".
  4. Sukses → generate token dummy `dummy-token-{id}-{unix}` + `UserResponse`.
     ⚠️ DUMMY: versi real = JWT bertanda tangan + expiry.
- **Response:**
  - `200 OK` → `BaseResponse{ data: LoginResponse{token,user} }`, message `"login success"`.
  - `400` code `VALIDATION_ERROR`.
  - `401` code `INVALID_CREDENTIALS`.
  - `500` code `INTERNAL_ERROR`.
- **Logging:** INFO `login: attempt` → INFO `login: success` | WARN `login: rejected, invalid
  credentials` | ERROR saat gagal lookup.

---

## Konvensi menulis entri baru

Untuk tiap endpoint, dokumentasikan minimal: **Tujuan · Auth · Request (+validasi) · Bisnis logic
(langkah + aturan/edge case) · Response (sukses + tiap error code) · Logging**. Tambahkan juga
baris di tabel "Ringkasan endpoint". Fokus pada *keputusan bisnis* (kenapa 409, kenapa generik,
aturan unik, transisi status, kuota, dll) — bukan sekadar schema (itu sudah di Swagger).
