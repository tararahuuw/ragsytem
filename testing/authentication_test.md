# Authentication API Test Playbook

**Module:** Authentication (register, login) — **dummy / in-memory**
**Status:** ✅ READY (register + login). Endpoint lanjutan (forgot/reset password, `/me`,
refresh) masih ⏳ PENDING — lihat bagian "Roadmap" di bawah.

> Catatan: implementasi saat ini **DUMMY** — user disimpan in-memory (hilang saat server
> restart), password belum di-hash, token masih `dummy-token-...` (bukan JWT). Playbook ini
> menguji kontrak & smoke, bukan keamanan produksi.

## Environment
- Base URL: `http://localhost:8080/api/v1`
- Prasyarat: server jalan (`make run`).
- Karena store in-memory: jalankan playbook pada instance server yang **baru start** agar TC-01
  (register) tidak bentrok, atau pakai email unik per run.

## Variables
| Nama | Sumber |
|---|---|
| `BASE_URL` | `http://localhost:8080/api/v1` |
| `EMAIL` | `qa+$(date +%s)@example.com` (unik per run) |
| `PASSWORD` | `secret123` |
| `TOKEN` | di-capture dari TC-03 (login) |

> Set di awal run:
> ```bash
> BASE_URL=http://localhost:8080/api/v1
> EMAIL="qa+$(date +%s)@example.com"
> PASSWORD='secret123'
> ```

---

## Test Cases

### TC-01 — Register user baru
- **Method / Path:** `POST /auth/register`
- **curl:**
  ```bash
  curl -s -i -X POST "$BASE_URL/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"QA User\",\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}"
  ```
- **Ekspektasi:**
  - Status `201`
  - `success == true`, `data.email == $EMAIL`, `data.id` terisi
  - **Password TIDAK muncul** di response

### TC-02 — Register email duplikat
- **Method / Path:** `POST /auth/register` (email sama seperti TC-01)
- **curl:**
  ```bash
  curl -s -o /dev/null -w "%{http_code}\n" -X POST "$BASE_URL/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"QA User\",\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}"
  ```
- **Ekspektasi:**
  - Status `409`; `success == false`; **bukan** `5xx`

### TC-03 — Login berhasil
- **Method / Path:** `POST /auth/login`
- **curl:**
  ```bash
  curl -s -i -X POST "$BASE_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}"
  ```
- **Capture:** `data.token` → `TOKEN`
- **Ekspektasi:**
  - Status `200`
  - `data.token` tidak kosong; `data.user.email == $EMAIL`

### TC-04 — Login password salah
- **Method / Path:** `POST /auth/login`
- **curl:**
  ```bash
  curl -s -o /dev/null -w "%{http_code}\n" -X POST "$BASE_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$EMAIL\",\"password\":\"salah\"}"
  ```
- **Ekspektasi:**
  - Status `401`; pesan generik (tidak membedakan email tak ada vs password salah); **bukan** `5xx`

### TC-05 — Validasi input register
- **Method / Path:** `POST /auth/register`
- **Body:** email invalid + password terlalu pendek (`min=6`)
- **curl:**
  ```bash
  curl -s -o /dev/null -w "%{http_code}\n" -X POST "$BASE_URL/auth/register" \
    -H "Content-Type: application/json" \
    -d '{"name":"x","email":"bukan-email","password":"1"}'
  ```
- **Ekspektasi:**
  - Status `400`; **bukan** `5xx`

### TC-06 — Login tanpa body / body kosong
- **Method / Path:** `POST /auth/login`
- **curl:**
  ```bash
  curl -s -o /dev/null -w "%{http_code}\n" -X POST "$BASE_URL/auth/login" \
    -H "Content-Type: application/json" -d '{}'
  ```
- **Ekspektasi:**
  - Status `400` (field `required` gagal); **bukan** `5xx`

---

## Roadmap (PENDING — belum ada endpoint)
Tambahkan case saat fitur dibangun:
- `POST /auth/forgot-password`, `POST /auth/reset-password`
- `GET /auth/me` (butuh middleware auth / Bearer token)
- `POST /auth/refresh`
- Ganti dummy → persistensi GORM (`users`), hash password (bcrypt), JWT asli.
