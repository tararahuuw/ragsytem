# Chat (Conversation / RAG Q&A) API Test Playbook

**Module:** chat — `POST /chat/ask`, `GET /chat/sessions`, `GET|DELETE /chat/sessions/{id}`.
AI masih **mock** (jawaban tiruan) sampai kontrak tim AI siap.
**Status:** ✅ READY

## Environment
- Base URL: `http://localhost:8080/api/v1`
- Prasyarat: server jalan, PostgreSQL up. (MinIO tak wajib untuk chat, tapi app butuh MinIO saat start.)
- Auth: butuh access token (login).

## Konsep
- `session_id` = **UUID dari client**. Id sama = lanjut percakapan; id baru = percakapan baru.
- Scope **per-user**; sliding window **20 sesi/user**.

## Setup
```bash
BASE_URL=http://localhost:8080/api/v1
TOK=$(curl -s -X POST "$BASE_URL/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"<user>","password":"<pass>"}' | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['access_token'])")
SID=$(uuidgen)
```

---

## Test Cases

### TC-01 — Ask #1 (buat session baru) → 200
```bash
curl -s -X POST "$BASE_URL/chat/ask" -H "Authorization: Bearer $TOK" -H "Content-Type: application/json" \
  -d "{\"session_id\":\"$SID\",\"question\":\"Ringkas dokumen laporan tahunan\"}"
```
- **Ekspektasi:** `200`; `data.session_id==$SID`; `data.answer` terisi (mock: memuat `organization=` dan `thread=`).

### TC-02 — Ask #2 (lanjut percakapan sama) → 200
Ulangi TC-01 dengan `$SID` sama, pertanyaan berbeda. **Ekspektasi:** `200`.

### TC-03 — List sessions → 200
```bash
curl -s "$BASE_URL/chat/sessions" -H "Authorization: Bearer $TOK"
```
- **Ekspektasi:** `200`; sesi `$SID` ada; `title` = potongan pertanyaan pertama.

### TC-04 — Get session detail → 200 (urutan pesan)
```bash
curl -s "$BASE_URL/chat/sessions/$SID" -H "Authorization: Bearer $TOK"
```
- **Ekspektasi:** `200`; `messages` = 4 (user, assistant, user, assistant) urut `created_at ASC`.

### TC-05 — Validasi & auth
| Sub | Kondisi | Ekspektasi |
|---|---|---|
| a | `session_id` invalid (mis. `../x`) | `400 INVALID_SESSION` |
| b | tanpa `question` | `400 VALIDATION_ERROR` |
| c | tanpa token | `401` |
| d | GET session tak ada | `404 SESSION_NOT_FOUND` |

### TC-06 — Security: kepemilikan sesi
Buat user lain (org apa pun), dapatkan token `OTOK`.
| Sub | Aksi | Ekspektasi |
|---|---|---|
| a | user lain `GET /chat/sessions/$SID` | `404` (tak bocorkan sesi orang) |
| b | user lain `DELETE /chat/sessions/$SID` | `404` |
| c | user lain `POST /chat/ask` dgn `session_id=$SID` | `404` (tak bisa reuse UUID orang) |
| d | pemilik `GET /chat/sessions/$SID` (setelah c) | `200` (sesi tetap utuh) |

### TC-07 — Delete (owner) → 200, lalu get → 404
```bash
curl -s -o /dev/null -w "%{http_code}\n" -X DELETE "$BASE_URL/chat/sessions/$SID" -H "Authorization: Bearer $TOK"
curl -s -o /dev/null -w "%{http_code}\n" "$BASE_URL/chat/sessions/$SID" -H "Authorization: Bearer $TOK"
```
- **Ekspektasi:** `200` lalu `404`.

### TC-08 — Rate limit `/chat/ask` → 429
Jalankan app dengan limit rendah (mis. `RATELIMIT_CHAT_PER_MIN=3 ./bin/ragsystem`), lalu kirim
`ask` beruntun > limit.
- **Ekspektasi:** N pertama `200`, sisanya `429` code `RATE_LIMITED` (per-user). Reset < 1 menit.

---

## Catatan
- **AI mock**: jawaban masih tiruan. Saat kontrak tim AI siap → ganti `ai.NewMockClient()` di
  `router/chat/route.go` dengan HTTP client asli (lihat CLAUDE.md §8c). Test kontrak tak berubah.
- Sliding window (20 sesi): untuk mengetes, buat 21 sesi (21 UUID berbeda) → sesi tertua otomatis hilang dari list.
