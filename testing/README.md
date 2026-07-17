# Testing Playbooks

Folder ini berisi **playbook test API** berformat Markdown. Satu file `.md` = satu **rumpun
endpoint** (mis. `authentication_test.md` mencakup register, login, forgot-password, dst).

Playbook = **spesifikasi test yang bisa dieksekusi Claude**. Claude membaca file, menjalankan tiap
request (via `curl`), membandingkan hasil dengan **Ekspektasi**, lalu melaporkan tabel PASS/FAIL.
Fokus utama: **smoke test** — deteksi endpoint mati / `5xx` / kontrak yang berubah.

---

## Cara Claude menjalankan playbook

Perintah dari user, contoh:
- `test module healthcheck` → jalankan `testing/healthcheck_test.md`.
- `test module authentication` → jalankan `testing/authentication_test.md`.
- `test semua module di testing` → jalankan semua playbook, satu per satu.

Langkah eksekusi (yang Claude lakukan):
1. **Pre-flight**: pastikan server hidup (`GET /api/v1/healthz` → `200`) dan Postgres up.
   Kalau server mati → hentikan, minta user `make run` dulu (jangan lapor semua FAIL palsu).
2. Baca **Environment** & **Variables** di playbook.
3. Jalankan tiap **Test Case** berurutan (TC-01, TC-02, ...). Case sering saling bergantung
   (mis. login butuh user hasil register) — hormati urutannya.
4. Untuk tiap case: kirim request, tangkap status + body, **capture** variabel bila diminti
   (mis. simpan `access_token`), bandingkan ke **Ekspektasi**.
5. Tandai **PASS** kalau semua assertion terpenuhi, **FAIL** kalau tidak. `5xx` yang tak
   diharapkan = **FAIL** dan disorot.
6. Akhiri dengan **laporan** (lihat format di bawah).

> Playbook = spec. Claude = runner + triage. Kalau ada FAIL, Claude menjelaskan penyebab
> singkat (status vs harapan, cuplikan body), bukan hanya "FAIL".

---

## Konvensi penulisan playbook

Setiap file mengikuti kerangka ini (lihat `_TEMPLATE.md`):

- **Module** — nama rumpun + status (`READY` / `PENDING — endpoint belum ada`).
- **Environment** — Base URL, prasyarat.
- **Variables** — nilai yang dibagikan antar-case (token, id, email unik, dll).
- **Test Cases** — daftar `TC-xx`, tiap case punya:
  - `Method` + `Path`
  - `Headers` (bila ada)
  - `Body` (bila ada)
  - `curl` — perintah konkret yang bisa langsung dijalankan
  - `Capture` — variabel yang diambil dari response (opsional)
  - `Ekspektasi` — status code + assertion body
- **Teardown** — pembersihan data uji bila perlu (opsional).

### Aturan penting
- **Data unik**: untuk endpoint yang membuat data (register), gunakan nilai unik agar
  bisa dijalankan berulang tanpa bentrok — mis. email `qa+<timestamp>@example.com`.
- **Idempotent bila bisa**: playbook sebaiknya bisa dijalankan berkali-kali.
- **Jangan hardcode secret**: ambil dari `.env` / environment, jangan tulis kredensial asli.
- **Base URL** default: `http://localhost:8080/api/v1`.

---

## Format laporan (output Claude)

```
## Hasil Test — <module> (<timestamp>)
Base URL: http://localhost:8080/api/v1

| Case  | Endpoint                | Status | Harapan | Hasil |
|-------|-------------------------|--------|---------|-------|
| TC-01 | GET /healthz            | 200    | 200     | PASS  |
| TC-02 | POST /auth/register     | 500    | 201     | FAIL  |

Ringkasan: 1 PASS, 1 FAIL
🔴 Sorotan 5xx: TC-02 POST /auth/register → 500 (…cuplikan error…)
```

---

## Daftar playbook

| File | Module | Status |
|---|---|---|
| `healthcheck_test.md` | Health check | READY |
| `authentication_test.md` | Auth (register/login/refresh) + User (me/get/update/role/soft-delete), JWT + RBAC + multi-tenant | READY |
| `upload_test.md` | Upload chunked file besar (PDF) → MinIO → compose → presigned | READY |

Tambahkan baris baru di tabel ini setiap kali membuat playbook baru.
