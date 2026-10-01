# DockWings

> Daemon Go buat DockPanel — kontrol Docker container per server game.

Dikembangkan oleh **Julak Junior** ([@Julakk](https://github.com/Julakk)). Repo terpisah dari [DockPanel](https://github.com/Julakk/DockPanel) (Laravel), sesuai pola Panel/Wings di Pterodactyl.

---

## Status: v0.2.0

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
internal/docker/         → interface Environment + StubEnvironment (belum ada Docker asli)
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

## Konfigurasi TLS (opsional)

```json
"ssl": {
  "enabled": true,
  "cert": "/etc/letsencrypt/live/NODE_FQDN/fullchain.pem",
  "key": "/etc/letsencrypt/live/NODE_FQDN/privkey.pem"
}
```

## Setup Development

Butuh Go 1.22+.

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
- [x] File manager API + SFTP server
- [x] TLS opsional + `/api/system` (v0.2.0)

## License

MIT
