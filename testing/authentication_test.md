# Authentication & User API Test Playbook (RBAC)

**Module:** auth (register[admin], login, refresh) + user (me, get, update, soft-delete[admin]).
JWT + multi-tenant + **RBAC (admin/user)**.
**Status:** ✅ READY

## Environment
- Base URL: `http://localhost:8080/api/v1`
- Prasyarat: server jalan (`make run`), PostgreSQL up, tabel `users` termigrasi.

## Bootstrap admin (WAJIB — sekali per DB)
Register kini **admin-only**, jadi admin pertama disiapkan **manual via SQL** (promote user yang
sudah ada; kalau DB kosong, insert dulu satu user lalu promote):
```bash
# promote user existing menjadi admin
psql ragsystem -c "UPDATE users SET role='admin' WHERE email='<email_user_existing>';"
# verifikasi
psql ragsystem -c "SELECT id,email,role FROM users WHERE role='admin';"
```
Catat kredensial admin ini (email + password aslinya) untuk login di TC-00.

## Variables
| Nama | Sumber |
|---|---|
| `ADMIN_EMAIL` / `ADMIN_PASS` | akun yang di-promote via SQL |
| `ADMIN_TOKEN` | access token hasil login admin (TC-00) |
| `USER_EMAIL` | `qa+$(date +%s)@pln.co.id` (dibuat admin) |
| `USER_TOKEN` | access token user biasa |
| `UID` | id user hasil register |

---

## Test Cases

### TC-00 — Login admin → 200 (bootstrap token)
```bash
curl -s -X POST "$BASE_URL/auth/login" -H "Content-Type: application/json" \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASS\"}"
```
- **Capture:** `data.access_token` → `ADMIN_TOKEN`.
- **Ekspektasi:** `200`; decode payload → `role:"admin"`.

### TC-01 — Register tanpa token → 401
```bash
curl -s -o /dev/null -w "%{http_code}\n" -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"QA\",\"email\":\"$USER_EMAIL\",\"password\":\"secret123\",\"organization_code\":\"pln\"}"
```
**Ekspektasi:** `401 UNAUTHORIZED`.

### TC-02 — Register oleh admin → 201 (role selalu user)
Sama seperti TC-01 + header `Authorization: Bearer $ADMIN_TOKEN`.
- **Capture:** `data.id` → `UID`.
- **Ekspektasi:** `201`; `data.role="user"`; password tak muncul.

### TC-03 — Register email duplikat (admin) → 409
Ulangi TC-02 email sama. **Ekspektasi:** `409 EMAIL_TAKEN`.

### TC-04 — Register validasi (admin) → 400
Body email invalid / password pendek / org kosong. **Ekspektasi:** `400 VALIDATION_ERROR`.

### TC-05 — Login user baru → 200
Login `$USER_EMAIL` / `secret123`. **Capture:** `data.access_token` → `USER_TOKEN`.
Decode payload → `role:"user"`.

### TC-06 — Register oleh user biasa → 403
Header `Authorization: Bearer $USER_TOKEN`. **Ekspektasi:** `403 FORBIDDEN_ROLE`
(RBAC: hanya admin boleh register).

### TC-07 — /users/me (user token) → 200
```bash
curl -s "$BASE_URL/users/me" -H "Authorization: Bearer $USER_TOKEN"
```
**Ekspektasi:** `200`; `data.email=$USER_EMAIL`, `data.role="user"`.

### TC-08 — /users/me tanpa token → 401.

### TC-09 — Update user (user token, self org) → 200
`PUT /users/$UID` body `{"name":"QA Updated"}`. **Ekspektasi:** `200`; name berubah.

### TC-10 — Delete oleh user biasa → 403
```bash
curl -s -o /dev/null -w "%{http_code}\n" -X DELETE "$BASE_URL/users/$UID" -H "Authorization: Bearer $USER_TOKEN"
```
**Ekspektasi:** `403 FORBIDDEN_ROLE` (delete admin-only).

### TC-11 — Delete oleh admin → 200 (soft delete)
```bash
curl -s -X DELETE "$BASE_URL/users/$UID" -H "Authorization: Bearer $ADMIN_TOKEN"
```
**Ekspektasi:** `200`. Lanjutan:
- `GET /users/$UID` (admin) → `404 USER_NOT_FOUND`.
- Login `$USER_EMAIL` → `401` (user terhapus).
- DB: baris `id=$UID` masih ada, `deleted_at` terisi.

### TC-12 — Admin global (lintas org) → 200
Register user org `icon` (via admin), lalu admin akses `GET /users/{id_icon}`.
**Ekspektasi:** `200` (admin bypass tenant; **bukan** 403). Bandingkan: user `pln` akses user
`icon` → `403 FORBIDDEN_ORGANIZATION`.

### TC-13 — Refresh → 200; refresh pakai access token → 401
`POST /auth/refresh` dengan refresh token → `200`; dengan access token → `401 INVALID_REFRESH_TOKEN`.

### TC-14 — Ubah role: gating & validasi (`PATCH /users/{id}/role`)
Target = user biasa `UID` (buat via admin dulu). Semua via header sesuai.
| Sub | Request | Ekspektasi |
|---|---|---|
| a | tanpa token | `401 UNAUTHORIZED` |
| b | token USER biasa, body `{"role":"admin"}` | `403 FORBIDDEN_ROLE` |
| c | admin, body `{"role":"superuser"}` | `400 INVALID_ROLE` |
| d | admin, ubah role **diri sendiri** | `400 CANNOT_CHANGE_OWN_ROLE` |
| e | admin, user id tak ada | `404 USER_NOT_FOUND` |

### TC-15 — Promote user → admin (end-to-end)
```bash
curl -s -X PATCH "$BASE_URL/users/$UID/role" -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" -d '{"role":"admin"}'
```
- **Ekspektasi:** `200`; `data.role="admin"`.
- **Bukti fungsional:** login ulang user tsb → token `role:"admin"` → sekarang **bisa** hit
  `POST /auth/register` (`201`) yang sebelumnya `403`.
- **Demote balik:** `{"role":"user"}` → `200`; login ulang → register kembali `403`.

---

## Teardown (opsional)
```sql
DELETE FROM users WHERE email LIKE 'qa%@%';
```

## Roadmap (PENDING)
- Endpoint ubah role (promote/demote via API) · forgot/reset password · revoke refresh token.
