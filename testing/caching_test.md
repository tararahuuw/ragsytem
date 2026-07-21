# Caching (Redis) API Test Playbook

**Module:** caching lintas-module (organization + document) via `internal/infra/cache`.
**Status:** ✅ READY

Menguji perilaku cache: **miss→set**, **hit**, **invalidasi saat write**, **isolasi tenant**, dan
**fail-open** (Redis mati → endpoint tetap jalan lewat DB). Cache = optimasi; kebenaran tetap di DB.

## Environment
- Base URL: `http://localhost:8080/api/v1` · server jalan, Postgres up.
- **Redis** up + `REDIS_ADDR=localhost:6379`, `CACHE_ENABLED=true` di `.env`. Startup log harus:
  `level=INFO msg="cache enabled (redis)"`. (Kalau `cache disabled` → set env & restart.)
- Tool: `redis-cli` (dari `brew install redis`) untuk inspeksi key.
- Butuh **admin token** (`ATOK`). Bootstrap: lihat `authentication_test.md` (atau seed admin +
  login). `TS=$(date +%s)`.

## Konvensi key (namespace `ragsystem:`)
`org:exists:<code>` · `org:get:<code>` · `org:list` · `doc:list:<org|__all__>` · `doc:id:<id>`.

---

## Test Cases

### TC-01 — Miss → Set (cache terisi setelah baca pertama)
```bash
redis-cli flushall
curl -s -o /dev/null -w "%{http_code}\n" "$BASE_URL/organizations" -H "Authorization: Bearer $ATOK"
redis-cli keys 'ragsystem:*'
```
- **Ekspektasi:** `200`; setelah GET, key `ragsystem:org:list` **muncul** (sebelumnya kosong).

### TC-02 — Hit + TTL (baca kedua dari cache, ada TTL)
```bash
curl -s -o /dev/null -w "%{http_code}\n" "$BASE_URL/organizations" -H "Authorization: Bearer $ATOK"
redis-cli ttl ragsystem:org:list
```
- **Ekspektasi:** `200`; TTL = angka positif (≈ `CACHE_TTL`, default 300 dtk). Response identik TC-01.

### TC-03 — ExistsActive di-cache (hot path register)
```bash
curl -s -o /dev/null -w "%{http_code}\n" -X POST "$BASE_URL/auth/register" -H "Authorization: Bearer $ATOK" \
  -H 'Content-Type: application/json' -d "{\"name\":\"x\",\"email\":\"c$TS@pln.co.id\",\"password\":\"secret123\",\"organization_code\":\"pln\"}"
redis-cli get ragsystem:org:exists:pln
```
- **Ekspektasi:** `201`; `ragsystem:org:exists:pln` = `1` (org ada & aktif → hasil validasi ter-cache).

### TC-04 — Invalidasi saat WRITE (create org → org:list terhapus)
```bash
redis-cli get ragsystem:org:list   # pastikan ADA (dari TC-01/02)
curl -s -o /dev/null -w "%{http_code}\n" -X POST "$BASE_URL/organizations" -H "Authorization: Bearer $ATOK" \
  -H 'Content-Type: application/json' -d "{\"code\":\"cch$TS\",\"name\":\"Cache Test\"}"
redis-cli get ragsystem:org:list   # harus (nil)
```
- **Ekspektasi:** create `201`; `ragsystem:org:list` **terhapus** (nil) → baca berikutnya ambil data
  fresh dari DB. Ini bukti korektness: perubahan langsung terlihat, bukan nunggu TTL.

### TC-05 — Update org invalidasi ExistsActive (keamanan)
```bash
curl -s -o /dev/null -X PUT "$BASE_URL/organizations/cch$TS" -H "Authorization: Bearer $ATOK" \
  -H 'Content-Type: application/json' -d '{"active":false}'
redis-cli get ragsystem:org:exists:cch$TS   # nil (ter-invalidasi)
# register ke org yg baru dinonaktifkan → harus 400 (bukan lolos dari cache basi)
curl -s -o /dev/null -w "%{http_code}\n" -X POST "$BASE_URL/auth/register" -H "Authorization: Bearer $ATOK" \
  -H 'Content-Type: application/json' -d "{\"name\":\"x\",\"email\":\"d$TS@x.co\",\"password\":\"secret123\",\"organization_code\":\"cch$TS\"}"
```
- **Ekspektasi:** key `org:exists:cch$TS` nil; register **`400 INVALID_ORGANIZATION`** (deactivate
  langsung berlaku, cache tak menyimpan status basi).

### TC-06 — Document list ter-cache + isolasi tenant (key mengandung org)
```bash
curl -s -o /dev/null "$BASE_URL/documents" -H "Authorization: Bearer $ATOK"   # admin → scope all
redis-cli keys 'ragsystem:doc:list:*'
```
- **Ekspektasi:** `200`; muncul key `ragsystem:doc:list:__all__` (admin). User biasa → key
  `ragsystem:doc:list:<org-nya>` (tenant tak berbagi list). *(Invalidasi list terjadi saat upload
  dokumen baru selesai — lihat `upload_test.md`.)*

### TC-07 — FAIL-OPEN (Redis mati → endpoint tetap jalan via DB) 🔑
```bash
redis-cli shutdown nosave    # matikan Redis sementara
for p in "/organizations" "/organizations/pln" "/documents"; do
  echo -n "$p → "; curl -s -o /dev/null -w "%{http_code}\n" "$BASE_URL$p" -H "Authorization: Bearer $ATOK"
done
# register (ExistsActive) juga harus jalan:
curl -s -o /dev/null -w "register → %{http_code}\n" -X POST "$BASE_URL/auth/register" -H "Authorization: Bearer $ATOK" \
  -H 'Content-Type: application/json' -d "{\"name\":\"x\",\"email\":\"fo$TS@pln.co.id\",\"password\":\"secret123\",\"organization_code\":\"pln\"}"
# nyalakan lagi:
redis-server --port 6379 --daemonize yes
```
- **Ekspektasi:** semua **`200`/`201`** walau Redis mati (fallback DB). Log server berisi WARN
  `... cache: ... (fail-open)` — **bukan** ERROR, dan **tidak** ada `5xx`. Bukti cache tak pernah
  jadi single-point-of-failure.

### TC-08 — Disabled cache (no-op) — opsional
Jalankan server dengan `CACHE_ENABLED=false` (atau `REDIS_ADDR=` kosong). Startup log:
`cache disabled ... no-op`. Semua endpoint tetap `200`, `redis-cli keys 'ragsystem:*'` tetap kosong
(tak ada yang ditulis). Bukti mockable.

---

## Catatan
- **Presigned URL tak di-cache**: hanya metadata dokumen; URL fresh tiap response (cek `preview_url`
  tetap valid walau metadata dari cache).
- Bersihkan namespace saat debugging: `redis-cli --scan --pattern 'ragsystem:*' | xargs redis-cli del`.
- Endpoint yang **tidak** di-cache (user/chat/auth-login/health) tak pernah menulis key `ragsystem:*`.
