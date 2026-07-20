# Organization API Test Playbook

**Module:** organization — CRUD tenant + guard `organization_code` (register/bulk validasi org).
**Status:** ✅ READY

## Environment
- Base URL: `http://localhost:8080/api/v1` · Prasyarat: server jalan, Postgres up.
- Butuh **admin token** (untuk create/update/delete) + user token (untuk cek RBAC/read).

## Setup
```bash
BASE_URL=http://localhost:8080/api/v1
ATOK=$(curl -s -X POST "$BASE_URL/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"<admin>","password":"<pass>"}' | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['access_token'])")
TS=$(date +%s)
```

---

## Test Cases

### TC-01 — Seed check → 200
`GET /organizations` → memuat `pln` & `icon` (di-seed dari users saat migrate).

### TC-02 — Create (admin) → 201
`POST /organizations` `{"code":"co$TS","name":"Co"}` → `201`.

### TC-03 — Create duplicate → 409
Ulangi TC-02 code sama → `409 ORG_EXISTS`.

### TC-04 — Create invalid code → 400
`{"code":"bad code!","name":"x"}` → `400 INVALID_CODE` (harus 2-64 alnum/-/_).

### TC-05 — Get / not found
`GET /organizations/co$TS` → `200`; `GET /organizations/nope` → `404 ORG_NOT_FOUND`.

### TC-06 — RBAC (write admin-only)
| Sub | Aksi | Ekspektasi |
|---|---|---|
| a | create tanpa token | `401` |
| b | create pakai token user biasa | `403 FORBIDDEN_ROLE` |
| c | list pakai token user biasa | `200` (read boleh) |

### TC-07 — Guard `organization_code` di register
| Sub | organization_code | Ekspektasi |
|---|---|---|
| a | `pln` (valid, aktif) | register `201` |
| b | `zzz$TS` (tak dikenal) | register `400 INVALID_ORGANIZATION` |
| c | `"pln "` (trailing space) | register `201` (trim bekerja) |
| d | bulk mix valid+invalid | partial: sukses utk valid, item invalid `INVALID_ORGANIZATION` |

### TC-08 — Deactivate memblokir registrasi
`PUT /organizations/icon {"active":false}` → `200`. Lalu register org `icon` → `400`. User
existing di `icon` **tetap** bisa login/akses. Reaktivasi: `{"active":true}` → `200`.

### TC-09 — Delete guard
| Sub | Aksi | Ekspektasi |
|---|---|---|
| a | delete `pln` (punya user) | `409 ORG_HAS_USERS` |
| b | create org kosong lalu delete | `200`, `GET` setelahnya `404` |
| c | register ke org yg baru dihapus | `400 INVALID_ORGANIZATION` |

---

## Catatan
- `organization_code` sekarang **bukan plain text bebas** — harus merujuk org yang ada & aktif.
- Read (list/get) untuk semua user; write (create/update/delete) admin-only.
