# DockWings

> Daemon Go buat DockPanel — kontrol Docker container per server game.

Dikembangkan oleh **Julak Junior** ([@Julakk](https://github.com/Julakk)). Repo terpisah dari [DockPanel](https://github.com/Julakk/DockPanel) (Laravel), sesuai pola Panel/Wings di Pterodactyl.

---

## Status: v0.5.1

Daemon udah punya kontrol Docker asli (`DockerEnvironment`), resource usage (CPU/RAM/disk), console WebSocket real-time, file manager API, dan dukungan port allocation. Mulai v0.2.0 ada TLS opsional dan endpoint `/api/system` buat versi daemon.

Catatan: Docker nggak bisa jalan di Termux/Android, jadi daemon ini harus dijalanin dan dites di VPS Linux.

### Scheme Node di Panel harus cocok sama config Wings

| `ssl.enabled` di Wings | Scheme Node di Panel |
| ---------------------- | -------------------- |
| `false` (default)      | `http`               |
| `true`                 | `https`              |

Kalau nggak cocok, Panel nampilin `cURL error 35 ... wrong version number`.

## Arsitektur

```
cmd/wings/main.go       → entry point
internal/config/        → load config.json
internal/api/            → HTTP routing + handlers + auth middleware
internal/server/         → model Server + Manager (registry in-memory)
internal/docker/         → interface Environment + DockerEnvironment asli (StubEnvironment buat test)
```

Kenapa `Environment` dibikin interface: biar logic HTTP handler bisa ditulis & ditest sekarang (pakai `StubEnvironment` yang cuma nge-log), terus nanti tinggal diganti `DockerEnvironment` asli tanpa ubah kode API sama sekali.

## Endpoint API

Semua endpoint butuh header `Authorization: Bearer {auth_token}` (token yang sama kayak `daemon_token` yang di-generate Panel pas bikin Node).

| Method | Path | Fungsi |
|---|---|---|
| GET | `/health` | Cek daemon hidup |
| POST | `/api/servers` | Bikin server baru (dipanggil pas provisioning) |
| POST | `/api/servers/{uuid}/power` | Start/stop/restart/kill |
| POST | `/api/servers/{uuid}/commands` | Kirim command ke console |
| DELETE | `/api/servers/{uuid}` | Hapus server |
| GET | `/api/system` | Versi daemon, OS, arsitektur, CPU, jumlah server |
| GET | `/api/servers/{uuid}/resources` | Resource usage (CPU/RAM/disk), uptime, dan network |
| PUT | `/api/servers/{uuid}/allocations` | Update port allocation container |
| POST | `/api/servers/{uuid}/install` | Jalankan install/reinstall server |
| GET | `/api/servers/{uuid}/install` | Status install (`idle`/`completed`/`failed`, `log`, `exit_code`) |
| POST | `/api/servers/{uuid}/backups` | Buat backup |
| GET | `/api/servers/{uuid}/backups/{backup}` | Status backup |
| GET | `/api/servers/{uuid}/backups/{backup}/download` | Download backup |
| DELETE | `/api/servers/{uuid}/backups/{backup}` | Hapus backup |
| POST | `/api/servers/{uuid}/backups/{backup}/restore` | Restore backup |
| GET | `/api/servers/{uuid}/restore` | Status restore |
| GET | `/api/servers/{uuid}/files/list` | List isi folder |
| GET | `/api/servers/{uuid}/files/contents` | Baca isi file |
| POST | `/api/servers/{uuid}/files/write` | Tulis file |
| POST | `/api/servers/{uuid}/files/mkdir` | Bikin folder |
| POST | `/api/servers/{uuid}/files/rename` | Rename/pindah file |
| POST | `/api/servers/{uuid}/files/delete` | Hapus file/folder |
| POST | `/api/servers/{uuid}/files/extract` | Ekstrak arsip .zip/.tar/.tar.gz/.tgz |
| POST | `/api/servers/{uuid}/files/compress` | Kompres file/folder jadi .zip |
| POST | `/api/servers/{uuid}/files/chmod` | Set izin executable (755) atau biasa (644) |
| POST | `/api/servers/{uuid}/files/pull` | Download file dari URL (http/https, IP publik saja) |
| GET | `/api/servers/{uuid}/ws/console` | WebSocket console real-time |

## SFTP

Aktif kalau `panel_url` diisi di `config.json` (contoh: `"panel_url": "https://panel.contoh.com"`). Login:
username `<email-akun>.<8 karakter pertama uuid server>`, password = password akun Panel. Port ikut `sftp_addr` (default `:2022`), jangan lupa dibuka di firewall.

## Konfigurasi TLS (opsional)

```json
"ssl": {
  "enabled": true,
  "cert": "/etc/letsencrypt/live/NODE_FQDN/fullchain.pem",
  "key": "/etc/letsencrypt/live/NODE_FQDN/privkey.pem"
}
```

## Setup Development

Butuh Go 1.26+ (sesuai `go.mod`).

```bash
git clone https://github.com/Julakk/DockWings.git
cd DockWings

cp config.example.json config.json
# edit config.json, isi auth_token sama daemon_token dari Panel

go build -o bin/wings ./cmd/wings
./bin/wings -config config.json
```

Testing:

```bash
go test ./...
```

⚠️ Development ini **wajib** di VPS/Linux beneran buat lanjut ke tahap integrasi Docker — nggak bisa dari Termux/Android sama sekali (beda dari DockPanel yang sebagian besar masih bisa dikerjain dari HP).

## Roadmap

- [x] Skeleton project + routing + auth middleware
- [x] Interface `Environment` + stub
- [x] `DockerEnvironment` asli
- [x] Resource usage reporting ke Panel
- [x] WebSocket console real-time
- [x] File manager API
- [x] SFTP server (v0.5.0)
- [x] TLS opsional + `/api/system` (v0.2.0)
- [x] Port allocation diterapkan ke container (v0.4.1)
- [x] Backup + restore lewat API (v0.4.1 sampai v0.4.2)
- [x] Install/reinstall server + status install tahan restart (v0.4.3 sampai v0.4.5)

## License

MIT
