# Health Check API Test Playbook

**Module:** Health check
**Status:** READY

## Environment
- Base URL: `http://localhost:8080/api/v1`
- Prasyarat: server jalan (`make run`), PostgreSQL up.

## Variables
| Nama | Nilai |
|---|---|
| `BASE_URL` | `http://localhost:8080/api/v1` |

---

## Test Cases

### TC-01 — Health check OK (DB up)
- **Method / Path:** `GET /healthz`
- **curl:**
  ```bash
  curl -s -i "$BASE_URL/healthz"
  ```
- **Ekspektasi:**
  - Status `200`
  - Body JSON: `{"status":"ok","database":"up"}`
  - Ada response header `X-Request-Id` (bukti middleware RequestID aktif)

### TC-02 — CORS preflight
- **Method / Path:** `OPTIONS /healthz`
- **Headers:** `Origin: http://localhost:3000`
- **curl:**
  ```bash
  curl -s -i -X OPTIONS "$BASE_URL/healthz" -H "Origin: http://localhost:3000"
  ```
- **Ekspektasi:**
  - Status `204`
  - Header `Access-Control-Allow-Origin: *`
  - Header `Access-Control-Allow-Methods` memuat `GET`

### TC-03 — Route tidak dikenal
- **Method / Path:** `GET /does-not-exist`
- **curl:**
  ```bash
  curl -s -i "$BASE_URL/does-not-exist"
  ```
- **Ekspektasi:**
  - Status `404`
  - **Bukan** `5xx` (server tidak crash untuk path asal)

### TC-04 — Swagger UI tersedia
- **Method / Path:** `GET /swagger/index.html` _(catatan: di root, di luar `/api/v1`)_
- **curl:**
  ```bash
  curl -s -o /dev/null -w "%{http_code}\n" "http://localhost:8080/swagger/index.html"
  ```
- **Ekspektasi:**
  - Status `200`

---

## Catatan (negatif / opsional)
Untuk memverifikasi jalur **DB down → 503** (jangan dijalankan di run smoke normal):
1. Matikan Postgres saat server tetap jalan: `brew services stop postgresql@16`.
2. `curl -s -i "$BASE_URL/healthz"` → **harus** `503` dengan body
   `{"status":"degraded","database":"down"}`.
3. Nyalakan lagi: `brew services start postgresql@16`.
