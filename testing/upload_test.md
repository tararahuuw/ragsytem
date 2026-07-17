# Upload (Chunked Large File) API Test Playbook

**Module:** upload — `POST /uploads/chunk` (chunked → MinIO → compose → presigned).
**Status:** ✅ READY

## Environment
- Base URL: `http://localhost:8080/api/v1`
- Prasyarat: server jalan (`make run`), PostgreSQL up, **MinIO up** (lihat "Menyiapkan MinIO").
- Auth: butuh access token user mana pun (via login).

## Menyiapkan MinIO (lokal)
```bash
brew install minio                      # sekali
MINIO_ROOT_USER=minioadmin MINIO_ROOT_PASSWORD=minioadmin \
  minio server /tmp/minio-data --address 127.0.0.1:9000 --console-address 127.0.0.1:9001 &
# app akan otomatis membuat bucket "ragsystem" saat start
```
Env app (default sudah cocok): `MINIO_ENDPOINT=localhost:9000`, key/secret `minioadmin`.

## Aturan kunci
- **PDF only.** Chunk **non-terakhir wajib ≥ 5 MiB** (batas S3/MinIO compose).
- `sessionId` stabil antar chunk; `chunkIndex` 0..totalChunks-1.

## Setup variabel
```bash
BASE_URL=http://localhost:8080/api/v1
TOK=$(curl -s -X POST "$BASE_URL/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"<user>","password":"<pass>"}' | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['access_token'])")
# buat PDF ~13MB & split 6MB
{ printf '%%PDF-1.4\n'; head -c 12999980 /dev/zero | tr '\0' 'A'; printf '\n%%%%EOF'; } > /tmp/up.pdf
SZ=$(stat -f%z /tmp/up.pdf); SHA=$(shasum -a 256 /tmp/up.pdf | awk '{print $1}')
( cd /tmp && rm -f ck_* && split -b 6000000 up.pdf ck_ )
SID=$(uuidgen)
```

---

## Test Cases

### TC-01 — Happy path: 3 chunks → merge
Kirim ck_aa (idx0), ck_ab (idx1), ck_ac (idx2), `totalChunks=3`, `chunkSize=6000000`, `fileSize=$SZ`, `sha256=$SHA`.
```bash
i=0; for f in ck_aa ck_ab ck_ac; do
  curl -s -X POST "$BASE_URL/uploads/chunk" -H "Authorization: Bearer $TOK" \
    -F "file=@/tmp/$f" -F "sessionId=$SID" -F "fileName=up.pdf" \
    -F "chunkIndex=$i" -F "totalChunks=3" -F "fileSize=$SZ" -F "chunkSize=6000000" -F "sha256=$SHA"; echo; i=$((i+1)); done
```
- **Ekspektasi:** chunk 0,1 → `200 upload_complete=false`; chunk 2 → `200 upload_complete=true`
  dengan `object_path` & `preview_url`.

### TC-02 — Integritas file (compose byte-perfect)
Download `preview_url` dari TC-01, bandingkan sha256 dengan file asli.
- **Ekspektasi:** sha256 download == `$SHA` (**MATCH**).

### TC-03 — Dedup SHA-256 → 409
Upload chunk 0 file lain dengan `sha256=$SHA` (yang sudah pernah selesai), sessionId baru.
- **Ekspektasi:** `409` code `DUPLICATE_FILE`. (Pakai `forceUpload=true` untuk bypass.)

### TC-04 — Non-PDF → 400
`file` berisi teks biasa, `fileName=bad.pdf`, `totalChunks=1`.
- **Ekspektasi:** `400` code `INVALID_FILE_TYPE` (MIME terdeteksi bukan application/pdf).

### TC-05 — Chunk < 5 MiB (multi-part) → 400
`totalChunks=3`, `chunkSize=1000000`.
- **Ekspektasi:** `400` code `CHUNK_TOO_SMALL`.

### TC-06 — Nama file path-traversal → 400
`fileName=../evil.pdf`.
- **Ekspektasi:** `400` code `INVALID_FILENAME`.

### TC-07 — Tanpa token → 401
Tanpa header Authorization.
- **Ekspektasi:** `401 UNAUTHORIZED`.

### TC-08 — Kuota habis → 429 (opsional)
Turunkan limit role di `upload_quota_configs` (dev), lalu upload melebihi limit.
- **Ekspektasi:** `429` code `QUOTA_EXCEEDED`.

---

## Verifikasi sisi data (opsional)
```bash
psql ragsystem -c "SELECT file_name,status,file_size,total_chunks,organization_code FROM upload_logs ORDER BY id DESC LIMIT 5;"
psql ragsystem -c "SELECT user_id,year_month,monthly_count,lifetime_count FROM upload_quota_usages;"
```
Chunk sementara di `temp_chunks/...` terhapus otomatis ~5 detik setelah merge.

## Catatan
- **Bulk** = banyak sesi chunk paralel (client fires multiple sessions). Server aman via state
  per-session + object storage.
- Ingest RAG (OCR/embedding) = langkah terpisah berikutnya (belum ada).
