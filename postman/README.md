# Postman — RAG System API

File yang bisa langsung di-import ke Postman (Import → pilih file):

| File | Isi |
|---|---|
| `ragsystem.postman_collection.json` | Koleksi request per-module (health, auth, …) |
| `ragsystem.local.postman_environment.json` | Environment lokal (`base_url`, `email`, `password`, `token`, …) |

## Cara pakai
1. **Import** kedua file di Postman.
2. Pilih environment **"RAG System - Local"** (pojok kanan atas).
3. Jalankan **auth → Login** → token otomatis tersimpan ke variable `{{token}}` (lihat tab Tests).
4. Endpoint terproteksi (nanti) tinggal pakai header `Authorization: Bearer {{token}}`.

## Variabel environment
| Var | Default | Keterangan |
|---|---|---|
| `base_url` | `http://localhost:8080/api/v1` | root API |
| `name` / `email` / `password` | John Doe / john@example.com / secret123 | kredensial contoh (dipakai di body register/login) |
| `token` | *(kosong)* | diisi otomatis oleh test script Login |

## Menjaga tetap sinkron
File ini adalah **artefak yang diperbarui bersama kode**: setiap ada endpoint baru atau perubahan
kontrak/bisnis logic, collection & environment ini ikut di-update (skill `rag-dev` Langkah 5d).
Alternatif: Swagger tersedia di `http://localhost:8080/swagger/doc.json` dan bisa di-import Postman,
tapi collection ini lebih kaya (folder rapi, contoh body, auto-capture token).
