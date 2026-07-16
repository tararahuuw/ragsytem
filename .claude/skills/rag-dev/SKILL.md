---
name: rag-dev
description: >-
  Prosedur baku pengembangan fitur end-to-end untuk backend RAG System (Go + Gin + GORM,
  arsitektur berlapis router→controller→service→repository). Pakai skill ini saat user minta
  menambah/mengubah endpoint atau fitur backend di project ragsytem: dari memahami maksud
  command, baca CLAUDE.md, telusuri file terdampak lintas-layer, inspeksi tabel DB (HANYA
  lokal/development — dilarang production), implementasi kode ikut pola sekitar, smoke test API
  terkait, update folder testing/, smoke test seluruh API 1 module, review security & business
  logic, update CLAUDE.md, lalu ringkas. Trigger: "kembangkan fitur", "tambah endpoint",
  "buat modul <x>", "ubah API <x>" di backend RAG.
---

# RAG System — Full Development Workflow

Skill ini menuntun pengembangan fitur backend **secara menyeluruh dan berurutan**. Jalankan
langkah 1–11 sesuai urutan. Jangan lompat langkah; kalau satu langkah gagal, hentikan dan lapor
sebelum lanjut.

Konteks project: Go 1.26 · Gin · GORM (PostgreSQL) · swaggo. Base URL `http://localhost:8080/api/v1`.
Root project: folder `ragsytem/`.

---

## Aturan umum (berlaku di semua langkah)
- **Ikuti pola file sekitar** — jangan memaksakan gaya baru. Baca file sejenis dulu.
- **Jangan commit/push** kecuali user bilang eksplisit ("review OK" / "commit").
- **Jaga dependency rule**: controller tak akses GORM langsung; service tak tahu HTTP.
- **Selalu regenerate Swagger** (`make swag`) bila anotasi/route berubah.
- **Verifikasi**: `go build ./...`, `go vet ./...`, `go test ./...` harus lolos.
- **Bahasa**: ikuti bahasa user (Indonesia).

---

## Langkah 1 — Pahami maksud & tujuan
Baca command user: fitur apa, endpoint apa, perilaku yang diharapkan, batasan.
**Jika belum jelas, TANYA DULU** (pakai pertanyaan singkat/terfokus) sebelum menyentuh kode.
Contoh yang wajib diklarifikasi bila ambigu: nama resource, field & tipe, aturan validasi,
auth/role yang butuh, format response. Jangan berasumsi diam-diam untuk keputusan yang mengubah
kontrak API.

## Langkah 2 — Baca konteks project
Baca `ragsytem/CLAUDE.md` (arsitektur, konvensi, resep tambah domain, status/roadmap) dan
`ragsytem/testing/README.md` (konvensi playbook). Pahami pola yang sudah ada sebelum menulis apa pun.

## Langkah 3 — Telusuri file terdampak (urut antar-layer)
Baca berurutan mengikuti aliran request:
`internal/router/router.go` → `internal/controller/<domain>_controller.go` →
`internal/service/<domain>_service.go` → `internal/repository/<domain>_repository.go`
(plus `internal/dto/<domain>_dto.go` dan `internal/model/<domain>.go`).
Untuk domain baru, jadikan slice `healthcheck` sebagai contoh pola end-to-end.
Petakan apa yang perlu dibuat/diubah di tiap layer sebelum implementasi.

## Langkah 4 — Inspeksi tabel DB terkait (READ-ONLY, LOKAL/DEV SAJA)
Boleh melihat struktur & isi tabel untuk memahami skema, **dengan syarat ketat**:
- **HANYA** database lokal/development. **DILARANG** menyentuh production.
- Verifikasi dulu: `DB_HOST` = `localhost`/`127.0.0.1` (atau service lokal) **dan** `APP_ENV` ≠
  `production`. Cek `.env` / environment. Kalau host/target bukan lokal atau env production →
  **tolak inspeksi**, lapor ke user, jangan konek.
- **Read-only saja**: `\dt`, `\d <table>`, `SELECT ... LIMIT`. **Tidak boleh** `INSERT/UPDATE/
  DELETE/ALTER/DROP`.
- Cara: `psql "$DB_NAME"` atau `psql -h localhost -U <user> -d <db>`. Contoh:
  ```bash
  psql ragsystem -c "\dt"
  psql ragsystem -c "\d users"
  psql ragsystem -c "SELECT * FROM users LIMIT 5;"
  ```
- Kalau tabel belum ada (fitur baru), catat rencana model + migrasi (`AutoMigrate` di
  `internal/database/database.go`).

## Langkah 5 — Implementasi kode
Implementasikan lintas-layer sesuai resep di CLAUDE.md §5 (model → migrasi → dto → repository →
service → controller → route+DI), **foldering per-module** (`internal/<layer>/<module>/`).
Repository & service pakai **interface + constructor `New...`**. Response umum pakai package
`internal/response` (`response.OK/Created/Error/ValidationError`), bukan `c.JSON` mentah.
**WAJIB pasang logging** (CLAUDE.md §4b): ambil `logger.FromContext(ctx)`, tulis INFO (attempt/
success) + WARN/ERROR di tiap cabang gagal, structured key–value, **tanpa data sensitif**,
selalu bawa `request_id`. Tulis anotasi Swagger di handler (regenerasi di Langkah 5b). Pastikan
`go build`/`vet`/`test` lolos.

## Langkah 5b — Update Swagger (bila perlu)
Setelah implementasi kode, **update dokumentasi Swagger jika ada perubahan yang mempengaruhinya**:
endpoint/route baru, perubahan method/path, request/response DTO, param, atau status code.
Langkah:
- Pastikan tiap handler baru/berubah punya anotasi swaggo (`// @Summary @Tags @Param @Success
  @Failure @Router ...`) dan model DTO ter-anotasi (`example:"..."` bila membantu).
- Regenerate: `make swag` (`swag init -g main.go -o docs`).
- Verifikasi `docs/` ter-update dan `go build ./...` tetap lolos (blank import `_ ".../docs"`).
- Cek cepat via UI `http://localhost:8080/swagger/index.html` bahwa endpoint muncul benar.
Kalau perubahan **tidak** menyentuh permukaan API (mis. refactor internal), lewati regenerasi dan
sebutkan alasannya di summary.

## Langkah 5c — Update ROUTES.md (bila ada API baru / perubahan bisnis logic)
Jika membuat endpoint baru **atau** mengubah bisnis logic endpoint yang ada, **wajib update
`ROUTES.md`** — katalog pengetahuan bisnis logic per-endpoint. Untuk tiap endpoint dokumentasikan:
**Tujuan · Auth · Request (+validasi) · Bisnis logic (langkah + aturan/edge case) · Response
(sukses + tiap error code) · Logging**, dan tambahkan/perbarui baris di tabel "Ringkasan endpoint".
Fokus pada *keputusan bisnis* (kenapa status/kode tertentu, aturan unik, transisi status, kuota,
anti-enumeration, dll) — bukan sekadar schema (itu domain Swagger). Kalau perubahan tidak
menyentuh endpoint/bisnis logic, lewati dan sebut alasannya di summary.

## Langkah 6 — Smoke test API terkait
Pastikan server jalan (`make run` di background) & Postgres up. Hit endpoint yang baru/diubah
dengan `curl`, cek status & body sesuai harapan. Perhatikan **tidak ada `5xx`** tak terduga.
Uji jalur sukses + minimal 1 jalur gagal (validasi/unauthorized).

## Langkah 7 — Update folder `testing/`
Buat/perbarui `testing/<module>_test.md` mengikuti `testing/_TEMPLATE.md` & konvensi
`testing/README.md`: tiap case punya method/path, curl konkret, capture, dan **Ekspektasi**.
Daftarkan/temukan playbook di tabel `testing/README.md` (status READY/PENDING). Pakai data unik
(mis. email `qa+<timestamp>`) agar bisa dijalankan berulang.

## Langkah 8 — Smoke test seluruh API 1 module
Jalankan **seluruh** playbook module terkait (semua `TC-xx`) berurutan, hormati dependensi antar
case. Bandingkan tiap hasil ke Ekspektasi. Hasilkan **tabel PASS/FAIL** + sorot `5xx`. Ini
memastikan perubahan tidak merusak endpoint serumpun lain.

## Langkah 9 — Review security & business logic
Cek minimal:
- **Security**: input validation, authz/role sesuai, tidak bocorkan data sensitif (password/hash/
  token) di response, SQL injection (pakai query GORM ter-parameter, bukan string concat),
  anti user-enumeration di flow auth, rate-limit/otorisasi bila relevan.
- **Business logic**: aturan domain benar (mis. saldo/kuota/status transisi), edge case (kosong,
  duplikat, nilai batas), konsistensi transaksi (pakai `db.Transaction` bila multi-write).
- **Logging & observability**: tiap cabang error ada log; INFO alur sukses ada; `request_id`
  terbawa; tidak ada data sensitif (password/token) yang ter-log.
- **Error handling (§4c)**: tiap `error` dicek & dipetakan (tidak ditelan); pakai sentinel +
  `errors.Is`; wrap `%w` saat menaikkan konteks; jangan `panic` untuk alur normal; goroutine
  baru punya `recover` sendiri. Panic request sudah tertutup `middleware.Recovery`.
Laporkan temuan; perbaiki yang jelas salah.

## Langkah 10 — Update CLAUDE.md
Perbarui `ragsytem/CLAUDE.md`: tambah/ubah entri domain, centang checklist §8 bila selesai, dan
tambahkan baris di **§9 Changelog keputusan** (tanggal + keputusan). Bila ada playbook baru,
pastikan tercatat di `testing/README.md`.

## Langkah 11 — Summary
Ringkas ke user: apa yang dibangun, file yang berubah (per-layer), hasil smoke test (tabel
PASS/FAIL), temuan security/business + statusnya, dan langkah lanjut yang disarankan. Sebutkan
dengan jujur bila ada yang di-skip atau gagal.

---

## Definition of Done
Fitur dianggap selesai bila: kode lolos build/vet/test, endpoint terkait & seluruh module lolos
smoke test (tanpa 5xx tak terduga), Swagger ter-generate, **ROUTES.md** & playbook `testing/`
terupdate, review security/business beres, dan CLAUDE.md diperbarui. Tunggu "review OK" dari user
sebelum commit/push.
