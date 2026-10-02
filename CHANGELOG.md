# Changelog

Semua perubahan penting di project ini dicatat di sini.

## [0.2.1] - 2026-10-03

> "Rilis kecil biar versi Wings sejalan sama Panel v0.10.0." 🐧

### Changed
- Versi daemon jadi 0.2.1. Nggak ada perubahan perilaku; kompatibel penuh dengan Panel v0.9.0 dan v0.10.0.

## [0.2.0] - 2026-10-02

> "Sekarang bisa HTTPS beneran, dan Panel bisa nanya versi daemon." 🐧

### Added
- Dukungan TLS opsional lewat blok `ssl` di `config.json` (`enabled`, `cert`, `key`), mirip `api.ssl` di Wings Pterodactyl. Kalau `enabled: true`, scheme Node di Panel harus `https`; kalau `false` (default), scheme Node harus `http`.
- Endpoint `GET /api/system` (butuh Bearer token): versi daemon, OS, arsitektur, jumlah CPU, dan jumlah server. Dipakai Panel v0.9.0+ buat nampilin versi dan status node.
- Versi daemon dipusatin di `internal/version` dan muncul di log pas startup. Bisa di-override pas build lewat `-ldflags`.
- Test buat `/api/system` (respon 200 dengan token, 401 tanpa token).

### Fixed
- Validasi config: `ssl.enabled` tanpa `cert`/`key` sekarang ditolak saat startup dengan pesan jelas.

## [0.1.0] - 2026-07-12

> "Belum bisa kontrol Docker beneran, tapi kerangkanya udah berdiri." 🐧

### ✨ Ditambahkan
- Skeleton project Go lengkap: `cmd/wings` (entry point), `internal/config`, `internal/server`, `internal/docker`, `internal/api`
- `Config` loader dari `config.json` (listen address, SFTP address, auth token, docker socket, data directory)
- Model `Server` + `Manager` — registry in-memory buat semua server yang dikelola daemon
- Interface `Environment` (abstraksi kontrol container) + `StubEnvironment` — implementasi sementara yang cuma logging, dipakai sampai integrasi Docker asli mulai dikerjain
- Routing HTTP API pakai `net/http.ServeMux` bawaan Go 1.22+, endpoint-nya dicocokin persis sama yang dipanggil `WingsService.php` di Panel:
  - `POST /api/servers` — bikin server baru
  - `POST /api/servers/{uuid}/power` — start/stop/restart/kill
  - `POST /api/servers/{uuid}/commands` — kirim command ke console
  - `DELETE /api/servers/{uuid}` — hapus server
  - `GET /health` — cek daemon hidup
- Middleware auth — validasi `Authorization: Bearer {token}` pakai `subtle.ConstantTimeCompare` (aman dari timing attack)
- CI otomatis (GitHub Actions): `go build`, `go vet`, `go test` — nggak butuh Docker buat CI ini karena semuanya masih pure Go + stub

### 📝 Catatan
- Ditulis 100% pakai Go standard library, sengaja nggak nambah dependency eksternal dulu (Docker SDK, dll) — biar nggak ada risiko compile/install issue sebelum ada VPS buat testing beneran
- Development belum bisa lanjut ke tahap `DockerEnvironment` asli, SFTP server, dan WebSocket console karena butuh Docker beneran — nggak bisa dari Termux/Android sama sekali (beda dari DockPanel yang sebagian besar masih bisa dikerjain dari HP)
- Repo ini terpisah dari [DockPanel](https://github.com/Julakk/DockPanel), sesuai pola Panel/Wings di Pterodactyl asli

## Roadmap Selanjutnya
- [ ] `DockerEnvironment` — integrasi Docker SDK asli (butuh VPS)
- [ ] SFTP server (butuh VPS)
- [ ] WebSocket console real-time (butuh VPS)
- [ ] Resource usage reporting (CPU/RAM/Disk) balik ke Panel (butuh VPS)
- [ ] Testing end-to-end `WingsService` (Panel) ↔ DockWings (butuh VPS)
- Tambah job CI integration test
