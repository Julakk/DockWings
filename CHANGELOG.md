# Changelog

Semua perubahan penting di project ini dicatat di sini.

## [0.5.1] - 2026-10-09
> "Panel bisa lihat uptime dan trafik network server." 🐧

### Added

- `GET /api/servers/{uuid}/resources` sekarang juga mengirim `uptime_ms` (sejak container start), `network_rx_bytes`, dan `network_tx_bytes` (total sejak container start, semua interface kecuali loopback). Nilainya 0 kalau server mati.

### Changed

- Versi daemon jadi 0.5.1.

## [0.5.0] - 2026-10-09
> "SFTP akhirnya beneran ada: login pakai akun Panel, terkurung di folder server." 🐧

### Added

- Server SFTP bawaan (port `sftp_addr`, default `:2022`). Login diverifikasi ke Panel lewat `POST /api/remote/sftp/auth`, jadi akun, password, dan hak subuser (read-only atau tulis) ikut aturan Panel.
- Tiap user dikurung di folder servernya dengan resolver yang sama seperti file manager: path `..` dan symlink ke luar folder ditolak. Symlink/hardlink nggak bisa dibuat, dan chmod dibatasi ke 0755/0644.
- Host key ed25519 dibikin otomatis di `/var/lib/dockwings/sftp_host_ed25519_key` (mode 600) dan dipertahankan antar-restart.
- Config baru `panel_url` (contoh `https://panel.contoh.com`). Kalau kosong, SFTP nggak dijalankan.
- Hanya subsystem `sftp`; shell, exec, dan port forwarding ditolak. Batas 128 koneksi bersamaan dan 30 detik buat handshake + login.

### Changed

- Versi daemon jadi 0.5.0. Dependency baru: `golang.org/x/crypto` dan `github.com/pkg/sftp`.

## [0.4.8] - 2026-10-08
> "Kompres, atur izin, dan download file dari URL langsung di server." 🐧

### Added

- `POST /api/servers/{uuid}/files/compress`: bikin arsip `.zip` dari file/folder (symlink dilewati, nggak menimpa arsip yang sudah ada, batas 1 GiB dan 20.000 file).
- `POST /api/servers/{uuid}/files/chmod`: jadikan file executable (0755) atau biasa (0644).
- `POST /api/servers/{uuid}/files/pull`: download file dari URL http/https ke server (maks 100 MB, maks 5 redirect). Alamat loopback, privat, link-local, CGNAT, dan IP host sendiri ditolak saat koneksi dibuat, jadi aman dari SSRF dan DNS rebinding.

### Changed

- Versi daemon jadi 0.4.8.

## [0.4.7] - 2026-10-08
> "Image buatan sendiri bisa dipakai, dan file hasil ekstrak langsung bisa dijalankan." 🐧

### Added

- Extract: file ELF dan script berawalan `#!` otomatis dapat bit executable. Zip dari Windows nggak bawa bit itu, jadi `samp03svr` hasil ekstrak langsung bisa jalan.

### Fixed

- Create container: kalau `docker pull` gagal tapi image sudah ada di host (image lokal yang nggak ada di registry), proses lanjut pakai image lokal, bukan error.

### Changed

- Versi daemon jadi 0.4.7.

## [0.4.6] - 2026-10-08
> "Zip dari panel sekarang bisa diekstrak langsung." 🐧

### Added

- Endpoint `POST /api/servers/{uuid}/files/extract` buat ngeluarin isi arsip `.zip`, `.tar`, `.tar.gz`, dan `.tgz` ke folder yang sama dengan arsipnya.
- Proteksi zip-slip (entry dengan `..` ditolak), batas total hasil ekstrak 1 GiB dan 20.000 file per arsip, symlink di dalam arsip dilewati, dan bit executable dipertahankan.
- Format `.rar` ditolak dengan pesan yang jelas.

### Changed

- Versi daemon jadi 0.4.6.

## [0.4.5] - 2026-10-07
> "Hasil install nggak hilang lagi pas daemon restart, dan Panel bisa lihat exit code script." 🐧

### Added

- Status install disimpan ke disk (`.installstate.json` di folder data, mode 600), jadi `GET /api/servers/{uuid}/install` tetap jawab `completed` atau `failed` beserta `log` setelah daemon restart. Sebelumnya balik ke `idle`.
- Kalau daemon mati pas install masih berjalan, statusnya jadi `failed` dengan pesan yang jelas (bukan `idle`), dan container install yang yatim dimatikan pas daemon start.
- Field `exit_code` di status install: exit code script (0 kalau sukses). Nggak ada kalau gagalnya bukan karena script (pull image gagal, timeout).

### Changed

- Versi daemon jadi 0.4.5.

## [0.4.4] - 2026-10-05
> "Script install egg yang chown folder script nggak lagi error." 🐧

### Fixed

- Folder script install (`/mnt/install`) sekarang bisa ditulis, nggak read-only lagi. Script egg Pterodactyl sering menjalankan `chown` di folder itu dan sebelumnya kena `Read-only file system`. Folder-nya sementara dan dihapus setelah install.

### Changed

- Versi daemon jadi 0.4.4.

## [0.4.3] - 2026-10-05
> "Reinstall server dari Panel akhirnya jalan: script install egg dieksekusi di container sementara." 🐧

### Added

- `POST /api/servers/{uuid}/install`: jalanin script install egg di container sementara (jalan di background, balas 202). Folder data server di-mount ke `/mnt/server`, script ke `/mnt/install/install.sh` (read-only), env variable server ikut diteruskan. Pakai `bash` kalau image-nya punya, kalau nggak `sh`. Server harus mati, kalau nggak balas 409. Batas waktu 30 menit.
- `GET /api/servers/{uuid}/install`: status install terakhir (`idle`, `running`, `completed`, `failed`) plus `error` dan ekor `log` kalau ada.
- Start, restart, hapus server, dan restore backup ditolak (409) selama install berjalan. Install juga ditolak selama restore berjalan.
- Pas daemon start, folder sementara sisa install yang terputus dibersihkan.

### Changed

- Versi daemon jadi 0.4.3.

### Catatan

- Script install bisa menimpa file server; Panel sudah minta konfirmasi sebelum manggil endpoint ini.
- Setelah daemon restart, status install balik ke `idle` (disimpan di memori). Panel menganggapnya gagal dan bisa di-set manual.

## [0.4.2] - 2026-10-04
> "Restore backup: aman, diverifikasi, dan nggak merusak data lama kalau gagal." 🐧

### Added

- `POST /api/servers/{uuid}/backups/{backup}/restore`: ganti seluruh isi folder data server dengan isi backup (jalan di background, balas 202). Server harus mati, kalau nggak balas 409. Arsip diverifikasi checksum SHA-256 dulu, diekstrak ke folder sementara, baru ditukar dengan folder asli. Kalau ada yang gagal, folder asli nggak disentuh. Entri arsip dengan path berbahaya (`..`, absolut) ditolak; symlink dan file spesial dilewati.
- `GET /api/servers/{uuid}/restore`: status restore terakhir (`idle`, `restoring`, `completed`, `failed`).
- Start dan restart ditolak (409) selama restore berjalan.
- Pas daemon start, sisa restore yang terputus dibereskan: folder asli dikembalikan kalau daemon sempat mati di tengah penukaran.

### Changed

- Versi daemon jadi 0.4.2.

### Catatan

- Restore butuh ruang disk sekitar dua kali ukuran data server selama proses (folder baru dan folder lama ada bersamaan).
- Restore mengganti SEMUA file server; file yang dibuat setelah backup ikut hilang.

## [0.4.1] - 2026-10-04
> "Port bisa diganti tanpa provision ulang, dan backup server akhirnya jalan." 🐧

### Added

- `PUT /api/servers/{uuid}/allocations`: simpan daftar allocation baru dari Panel lalu bikin ulang container supaya port mapping dan `SERVER_IP`/`SERVER_PORT` ikut berubah. Server yang lagi jalan direstart sebentar; folder data tetap utuh. Container lama dicadangkan sementara dan dikembalikan kalau pembuatan ulang gagal.
- Backup server: `POST /api/servers/{uuid}/backups` (jalan di background, balas 202), `GET /api/servers/{uuid}/backups/{backup}` (status `creating`/`completed`/`failed`, ukuran, checksum SHA-256), `GET .../download`, dan `DELETE`. Arsip `tar.gz` dari folder data, disimpan di `backup_directory`. Symlink dan file spesial dilewati.
- Setelan `backup_directory` di `config.json` (default `/var/lib/dockwings/backups`, nggak wajib diisi).

### Changed

- Versi daemon jadi 0.4.1.

### Catatan

- Restore backup belum ada. Arsip bisa di-download dan dibuka manual.
- Backup server yang lagi jalan nggak dijamin konsisten (file bisa berubah selama dibaca).
- Backup yang sedang dibuat saat daemon restart ditandai `failed`.
- Perubahan port butuh Panel v0.15.3+ supaya tombol Make Primary dan Hapus allocation ikut menerapkannya.

## [0.4.0] - 2026-10-03
> "Server nyala lagi sendiri setelah VPS reboot." 🐧

### Added

- Restart policy `unless-stopped` dipasang ke container tiap kali server di-Start (`docker update --restart=unless-stopped`). Server yang lagi jalan nyala lagi otomatis setelah reboot host atau restart Docker; server yang dihentikan lewat Stop atau Kill tetap mati.

### Changed

- Versi daemon jadi 0.4.0.

### Catatan

- Container yang sudah ada perlu `docker update --restart=unless-stopped <nama>` sekali; container baru dapat policy saat pertama kali di-Start.
- Server yang belum pernah di-Start sengaja nggak dapat policy, biar nggak jalan sendiri setelah reboot.
- Server yang crash juga dinyalakan lagi oleh Docker (dengan jeda bertahap); status di Panel belum tentu ikut sinkron.

## [0.3.0] - 2026-10-03
> "Akhirnya kontrol Docker beneran, bukan stub lagi." 🐧

### Added

- `DockerEnvironment` asli di `internal/docker/docker_env.go`, mengontrol container lewat Docker CLI (exec), menggantikan `StubEnvironment` sebagai implementasi utama. Aksi start/stop/restart/kill dan kirim command ke container server game sekarang jalan beneran.
- WebSocket console real-time: streaming log container dan kirim command dari satu koneksi.
- Endpoint `GET /api/servers/{uuid}/resources` buat resource bar Panel: CPU, memory (lewat `docker stats`), dan disk (lewat `du`).
- Port allocation: port server dipetakan ke container lewat `-p`, dan `SERVER_IP`/`SERVER_PORT` di-inject sebagai environment variable.
- File manager API: list, baca isi, tulis, mkdir, rename, dan hapus, dengan proteksi path traversal dan symlink (`internal/files`).
- Integration test lifecycle `DockerEnvironment` dan job CI buat integration test Docker.

### Changed

- Hardening container: validasi uuid dan image sebelum container dibuat.
- Versi daemon jadi 0.3.0.

### Catatan

- Perlu Docker terpasang di node dan akses ke Docker socket.
- File manager baru lewat HTTP API; server SFTP belum ada.
- Test end-to-end Panel ↔ DockWings belum dikerjakan.

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
- [x] `DockerEnvironment` — kontrol Docker lewat CLI (exec)
- [ ] SFTP server (butuh VPS)
- [x] WebSocket console real-time
- [x] Resource usage reporting (CPU/RAM/Disk) ke Panel
- [ ] Testing end-to-end `WingsService` (Panel) ↔ DockWings (butuh VPS)
- Tambah job CI integration test
