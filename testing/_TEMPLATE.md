# <Module> API Test Playbook

> Copy file ini menjadi `<module>_test.md`, isi tiap bagian, lalu daftarkan di `README.md`.

**Module:** <nama rumpun>
**Status:** READY | PENDING — endpoint belum ada

## Environment
- Base URL: `http://localhost:8080/api/v1`
- Prasyarat: server jalan (`make run`), PostgreSQL up.

## Variables
Nilai yang dibagikan antar-case (diisi/di-capture saat run):

| Nama | Contoh / Sumber |
|---|---|
| `BASE_URL` | `http://localhost:8080/api/v1` |
| `EMAIL` | `qa+<timestamp>@example.com` (unik per run) |
| `ACCESS_TOKEN` | di-capture dari TC login |

---

## Test Cases

### TC-01 — <nama case>
- **Method / Path:** `GET /path`
- **Headers:** `Content-Type: application/json`
- **Body:**
  ```json
  { }
  ```
- **curl:**
  ```bash
  curl -s -i -X GET "$BASE_URL/path"
  ```
- **Capture:** _(opsional)_ simpan `data.id` → `SOME_ID`
- **Ekspektasi:**
  - Status `200`
  - Body: `success == true`

### TC-02 — <nama case>
...

---

## Teardown _(opsional)_
- Hapus data uji yang dibuat, mis. `DELETE /path/{id}`.
