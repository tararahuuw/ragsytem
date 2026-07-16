# Authentication & User API Test Playbook

**Module:** auth (register, login, refresh) + user (me, get, update, soft delete) — JWT, multi-tenant.
**Status:** ✅ READY

## Environment
- Base URL: `http://localhost:8080/api/v1`
- Prasyarat: server jalan (`make run`), PostgreSQL up, tabel `users` termigrasi.
- Data unik per run: pakai email ber-timestamp agar bisa diulang.

## Variables
| Nama | Sumber |
|---|---|
| `BASE_URL` | `http://localhost:8080/api/v1` |
| `EMAIL` | `qa+$(date +%s)@pln.co.id` |
| `EMAIL2` | `qa2+$(date +%s)@icon.id` (org berbeda) |
| `PASSWORD` | `secret123` |
| `ACCESS` / `REFRESH` | di-capture dari Login |
| `UID` / `UID2` | id user hasil register |

> Setup:
> ```bash
> BASE_URL=http://localhost:8080/api/v1
> EMAIL="qa+$(date +%s)@pln.co.id"; EMAIL2="qa2+$(date +%s)@icon.id"; PASSWORD=secret123
> ```

---

## Test Cases

### TC-01 — Register (org pln) → 201
```bash
curl -s -i -X POST "$BASE_URL/auth/register" -H "Content-Type: application/json" \
  -d "{\"name\":\"QA\",\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"organization_code\":\"pln\"}"
```
- **Capture:** `data.id` → `UID`.
- **Ekspektasi:** `201`; `data.organization_code=pln`; **password tidak** muncul di body.

### TC-02 — Register email duplikat → 409
Ulangi TC-01 dengan `$EMAIL` sama. **Ekspektasi:** `409` code `EMAIL_TAKEN`; bukan 5xx.

### TC-03 — Register validasi → 400
Body `{"name":"x","email":"bad","password":"1","organization_code":""}`.
**Ekspektasi:** `400` code `VALIDATION_ERROR`.

### TC-04 — Login → 200 + token
```bash
curl -s -X POST "$BASE_URL/auth/login" -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}"
```
- **Capture:** `data.access_token` → `ACCESS`, `data.refresh_token` → `REFRESH`.
- **Ekspektasi:** `200`; kedua token non-kosong; `expires_in` > 0.
- **Cek claim:** decode bagian payload access token → memuat `organization_code:"pln"`.

### TC-05 — Login password salah → 401
Body password salah. **Ekspektasi:** `401` code `INVALID_CREDENTIALS` (generik).

### TC-06 — /users/me tanpa token → 401
```bash
curl -s -o /dev/null -w "%{http_code}\n" "$BASE_URL/users/me"
```
**Ekspektasi:** `401` code `UNAUTHORIZED`.

### TC-07 — /users/me dengan token → 200
```bash
curl -s "$BASE_URL/users/me" -H "Authorization: Bearer $ACCESS"
```
**Ekspektasi:** `200`; `data.email=$EMAIL`, `data.organization_code=pln`.

### TC-08 — Tenant isolation → 403
Register user kedua di org lain (`icon`), dapatkan `UID2`. Lalu akses lintas-org:
```bash
curl -s -o /dev/null -w "%{http_code}\n" "$BASE_URL/users/$UID2" -H "Authorization: Bearer $ACCESS"
```
**Ekspektasi:** `403` code `FORBIDDEN_ORGANIZATION` (user pln tak boleh lihat user icon).

### TC-09 — Update user → 200
```bash
curl -s -X PUT "$BASE_URL/users/$UID" -H "Authorization: Bearer $ACCESS" \
  -H "Content-Type: application/json" -d '{"name":"QA Updated"}'
```
**Ekspektasi:** `200`; `data.name=QA Updated`; email/org tak berubah.

### TC-10 — Refresh → 200
```bash
curl -s -o /dev/null -w "%{http_code}\n" -X POST "$BASE_URL/auth/refresh" \
  -H "Content-Type: application/json" -d "{\"refresh_token\":\"$REFRESH\"}"
```
**Ekspektasi:** `200` + token pair baru.

### TC-11 — Refresh pakai ACCESS token → 401
Kirim `$ACCESS` sebagai `refresh_token`. **Ekspektasi:** `401 INVALID_REFRESH_TOKEN`
(type guard: access token bukan refresh).

### TC-12 — Soft delete → 200, lalu efeknya
```bash
curl -s -o /dev/null -w "%{http_code}\n" -X DELETE "$BASE_URL/users/$UID" -H "Authorization: Bearer $ACCESS"
```
**Ekspektasi:** `200`. Lanjutan:
- `GET /users/$UID` (token masih ada) → `404 USER_NOT_FOUND`.
- Login `$EMAIL` lagi → `401` (user terhapus tak ditemukan).

---

## Teardown (opsional)
```sql
DELETE FROM users WHERE email LIKE 'qa%@%';   -- hard delete data uji
```

## Roadmap (PENDING)
- Forgot/reset password · role/authorization (admin) · rotasi/revoke refresh token.
