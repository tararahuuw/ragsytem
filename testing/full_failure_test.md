# Full Failure Test — semua API dalam kondisi error (+ verifikasi Sentry)

**Module:** lintas-module (auth · user · organization · upload · document · chat) + **debug** (Sentry).
**Status:** READY.

Playbook ini **khusus menguji jalur GAGAL** tiap API dan memverifikasi bahwa error server
benar-benar **tercapture ke Sentry**. Berbeda dari playbook per-module (yang fokus happy-path +
1 jalur gagal), file ini menabrak **setiap kategori error** (400/401/403/404/409/429/500) dan
menandai mana yang masuk Sentry.

---

## Kebijakan capture Sentry (WAJIB dipahami sebelum menilai hasil)

Error dikirim ke Sentry lewat **slog handler** (`internal/logger/sentry.go`): setiap log level
**≥ `SENTRY_LEVEL`** (default `error`) di-forward. Konsekuensinya:

| Kategori | Contoh | Log level | → Sentry (default `error`) |
|---|---|---|---|
| **5xx / panic** | INTERNAL_ERROR, panic recovered | `ERROR` | ✅ **ya** (exception) |
| **Error tak terduga** (DB/MinIO/AI down) | dependency error | `ERROR` | ✅ **ya** |
| **4xx klien** | 400/401/403/404/409 validasi/authz | `WARN` | ❌ **tidak** (by design, anti-noise) |
| **429 rate limit** | RATE_LIMITED | `WARN` | ❌ tidak |

> **Kenapa 4xx tidak ke Sentry?** Itu error *yang diharapkan* (klien salah kirim) — kalau ikut
> dikirim, Sentry banjir noise. Untuk menaikkan ambang (mis. ikut tangkap 4xx), set
> `SENTRY_LEVEL=warn` lalu restart (lihat **TC-F13**).

**Prasyarat Sentry aktif:** `SENTRY_DSN` terisi di `.env`. Bila kosong → integrasi mati (no-op),
kolom "→ Sentry" jadi tidak berlaku (error tetap di-log lokal). Cek baris startup:
`level=INFO msg="sentry enabled"`.

---

## Environment
- **Base URL:** `http://localhost:8080/api/v1` (`$BASE_URL`).
- Server hidup (`make run`) + Postgres up. `SENTRY_DSN` terisi.
- Debug endpoint **hanya** hidup saat `APP_ENV != production`.

## Variables
| Var | Cara isi |
|---|---|
| `BASE_URL` | `http://localhost:8080/api/v1` |
| `USER_TOKEN` | (opsional, untuk TC-F08/F09) access token user biasa — dari `authentication_test.md` TC-05 |
| `SLOG` | path file log server (untuk verifikasi baris ERROR) |

---

## Bagian A — Verifikasi pipeline Sentry (debug endpoints) → **✅ masuk Sentry**

Ini cara **paling andal** membuktikan error API sampai ke Sentry. Endpoint sengaja memicu error.

### TC-F01 — `GET /debug/error` → 500 + Sentry (exception)
```bash
curl -s -o /dev/null -w "%{http_code}\n" "$BASE_URL/debug/error"
```
- **Ekspektasi:** `500`, body `{code: DEBUG_FORCED_ERROR}`. Log server: `level=ERROR msg="debug:
  forced error"`. **→ Sentry:** 1 exception "forced debug error…" (level=error, tag `request_id`).

### TC-F02 — `GET /debug/panic` → 500 + Sentry (exception, via Recovery)
```bash
curl -s -o /dev/null -w "%{http_code}\n" "$BASE_URL/debug/panic"
```
- **Ekspektasi:** `500`, body `{code: INTERNAL_ERROR}` (envelope standar dari `middleware.Recovery`,
  stack **tidak** bocor ke klien). Log: `level=ERROR msg="panic recovered"` + `stack=...`.
  **→ Sentry:** 1 exception "panic: forced debug panic…".

### TC-F03 — `GET /debug/message` → 200, **tidak** ke Sentry (default level=error)
```bash
curl -s -o /dev/null -w "%{http_code}\n" "$BASE_URL/debug/message"
```
- **Ekspektasi:** `200`. Log: `level=WARN msg="debug: forced warn message"`.
  **→ Sentry:** ❌ tidak (WARN < error). Akan berubah jadi ✅ di **TC-F13** (`SENTRY_LEVEL=warn`).

### TC-F04 — Debug endpoint mati di production
- Jalankan `APP_ENV=production ./bin/ragsystem` → `GET /debug/error` → **`404`** (route tak
  di-mount). Verifikasi guard non-prod. *(Opsional — butuh restart mode prod.)*

---

## Bagian B — Error klien per-module → **❌ tidak ke Sentry** (di-log WARN)

Membuktikan 4xx *tidak* membanjiri Sentry. Semua di bawah harus **bukan 5xx**.

### TC-F05 — auth: kredensial & validasi
```bash
# login salah password → 401
curl -s -o /dev/null -w "login-wrong: %{http_code}\n" -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" -d '{"email":"nobody@x.com","password":"salah"}'
# body tak valid (email kosong) → 400
curl -s -o /dev/null -w "login-badbody: %{http_code}\n" -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" -d '{"email":"","password":""}'
# register tanpa token → 401
curl -s -o /dev/null -w "register-noauth: %{http_code}\n" -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" -d '{"name":"x","email":"x@x.com","password":"secret123","organization_code":"pln"}'
# reset-password token ngawur → 400
curl -s -o /dev/null -w "reset-badtoken: %{http_code}\n" -X POST "$BASE_URL/auth/reset-password" \
  -H "Content-Type: application/json" -d '{"token":"deadbeef","new_password":"secret123"}'
```
- **Ekspektasi:** `401`, `400`, `401`, `400`. **→ Sentry:** ❌ semua (WARN).
- **Catatan:** `forgot-password` selalu `200` (anti-enumeration) — bukan error.

### TC-F06 — user: authz & not-found
```bash
# akses /users/me tanpa token → 401
curl -s -o /dev/null -w "me-noauth: %{http_code}\n" "$BASE_URL/users/me"
# token ngawur → 401
curl -s -o /dev/null -w "me-badtoken: %{http_code}\n" "$BASE_URL/users/me" -H "Authorization: Bearer not.a.jwt"
```
- **Ekspektasi:** `401`, `401`. **→ Sentry:** ❌.

### TC-F07 — organization / chat / upload / document: butuh auth → 401 tanpa token
```bash
for p in "GET /organizations" "GET /documents" "GET /chat/sessions" "POST /uploads/chunk"; do
  m=${p% *}; path=${p#* };
  curl -s -o /dev/null -w "$path : %{http_code}\n" -X $m "$BASE_URL$path"
done
```
- **Ekspektasi:** semua `401` (protected route, tanpa Bearer). **→ Sentry:** ❌.

### TC-F08 — (auth) chat/document not-found & ownership → 404 *(butuh `USER_TOKEN`)*
```bash
curl -s -o /dev/null -w "chat-404: %{http_code}\n" "$BASE_URL/chat/sessions/00000000-0000-0000-0000-000000000000" -H "Authorization: Bearer $USER_TOKEN"
curl -s -o /dev/null -w "doc-404: %{http_code}\n"  "$BASE_URL/documents/99999999" -H "Authorization: Bearer $USER_TOKEN"
```
- **Ekspektasi:** `404` (DOCUMENT_NOT_FOUND / session bukan milik user). **→ Sentry:** ❌.

### TC-F09 — (auth) upload chunk body tak valid → 400 *(butuh `USER_TOKEN`)*
```bash
curl -s -o /dev/null -w "upload-badreq: %{http_code}\n" -X POST "$BASE_URL/uploads/chunk" \
  -H "Authorization: Bearer $USER_TOKEN" -F "foo=bar"
```
- **Ekspektasi:** `400` (field wajib hilang / bukan PDF). **→ Sentry:** ❌.

### TC-F10 — rate limit → 429 (per-IP, kategori `auth`)
```bash
for i in $(seq 1 25); do curl -s -o /dev/null -w "%{http_code} " -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" -d '{"email":"x@x.com","password":"x"}'; done; echo
```
- **Ekspektasi:** sebagian awal `401`, sisanya `429` (`RATE_LIMITED`) — jalankan dengan
  `RATELIMIT_AUTH_PER_MIN` rendah bila perlu. **→ Sentry:** ❌ (WARN).

---

## Bagian C — Error server nyata dari API bisnis → **✅ masuk Sentry** (opsional, advanced)

Membuktikan bahwa `5xx` *dari endpoint asli* (bukan debug) juga tertangkap — dipicu dengan
mematikan dependency.

### TC-F11 — MinIO mati → endpoint yang butuh object storage → 500 + Sentry
```bash
# 1. Matikan MinIO (mis. hentikan proses minio / docker stop).
# 2. Hit endpoint yang mem-presign / compose, mis. detail dokumen (butuh USER_TOKEN + id valid):
curl -s -o /dev/null -w "doc-detail-minio-down: %{http_code}\n" "$BASE_URL/documents/<ID_VALID>" -H "Authorization: Bearer $USER_TOKEN"
# 3. Nyalakan MinIO lagi.
```
- **Ekspektasi:** `500` (INTERNAL_ERROR) saat presign gagal. Log `level=ERROR`. **→ Sentry:** ✅
  exception dependency error. *(Skip bila tak ingin mengganggu infra; TC-F01/F02 sudah cukup
  membuktikan pipeline.)*

### TC-F12 — Postgres mati saat runtime → API DB-bound → 500 + Sentry
- Serupa TC-F11 tapi hentikan Postgres, lalu hit mis. `GET /users/me` (Bearer valid) → `500`,
  log ERROR, **→ Sentry ✅**. *(Advanced; kembalikan DB setelah tes.)*

---

## Bagian D — Toggle ambang level

### TC-F13 — `SENTRY_LEVEL=warn` → 4xx & WARN ikut ke Sentry
```bash
# restart server dengan level warn:
SENTRY_LEVEL=warn make run   # atau SENTRY_LEVEL=warn ./bin/ragsystem
```
- Ulangi **TC-F03** (`/debug/message`) → sekarang **→ Sentry ✅** (WARN ter-forward).
- Ulangi **TC-F05** (login salah) → 401 kini **→ Sentry ✅**.
- **Bukti:** ambang capture benar-benar dikontrol `SENTRY_LEVEL`. Kembalikan ke `error` setelah tes.

---

## Menjalankan cepat (satu kali)
```bash
BASE_URL=http://localhost:8080/api/v1
# pipeline Sentry (✅):
curl -s -o /dev/null -w "F01 /debug/error : %{http_code}\n" "$BASE_URL/debug/error"
curl -s -o /dev/null -w "F02 /debug/panic : %{http_code}\n" "$BASE_URL/debug/panic"
curl -s -o /dev/null -w "F03 /debug/message: %{http_code}\n" "$BASE_URL/debug/message"
# 4xx (❌ Sentry):
curl -s -o /dev/null -w "F06 /users/me noauth: %{http_code}\n" "$BASE_URL/users/me"
```
Lalu buka **Sentry → Issues**: harus muncul "forced debug error" & "panic: forced debug panic"
(environment sesuai `SENTRY_ENVIRONMENT`/`APP_ENV`), **tanpa** event untuk 401/404/400.

## Ekspektasi ringkas (tabel target)
| Case | Trigger | Status | → Sentry |
|---|---|---|---|
| F01 | GET /debug/error | 500 | ✅ |
| F02 | GET /debug/panic | 500 | ✅ |
| F03 | GET /debug/message | 200 | ❌ (✅ bila level=warn) |
| F05 | auth login/validasi | 401/400 | ❌ |
| F06 | users/me noauth | 401 | ❌ |
| F07 | protected routes noauth | 401 | ❌ |
| F08 | chat/doc not-found | 404 | ❌ |
| F09 | upload bad body | 400 | ❌ |
| F10 | login flood | 429 | ❌ |
| F11 | MinIO down + doc detail | 500 | ✅ |
| F13 | level=warn re-run F03/F05 | — | ✅ |
