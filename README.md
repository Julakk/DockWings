# DockWings

> Daemon Go buat DockPanel — kontrol Docker container per server game.

Dikembangkan oleh **Julak Junior** ([@Julakk](https://github.com/Julakk)). Repo terpisah dari [DockPanel](https://github.com/Julakk/DockPanel) (Laravel), sesuai pola Panel/Wings di Pterodactyl.

---

## Status: Skeleton Awal ⚠️

Repo ini baru berisi **struktur dasar & interface**, belum ada implementasi Docker beneran. Alasannya: development DockPanel sepenuhnya dari HP via Termux, dan **Docker nggak bisa jalan di Termux/Android**. Jadi bagian ini nunggu VPS tersedia buat development & testing lanjutan.

Yang udah ada sekarang:
- ✅ Struktur project & routing HTTP API
- ✅ Interface `Environment` (abstraksi kontrol container) + implementasi stub (cuma logging)
- ✅ Auth middleware (validasi token dari Panel)
- ✅ Endpoint API yang cocok sama `WingsService.php` di Panel
- ✅ CI (build + vet + test) via GitHub Actions

Yang belum:
- ❌ Implementasi Docker asli (`DockerEnvironment`) — butuh Docker SDK + VPS buat testing
- ❌ SFTP server
- ❌ WebSocket console real-time
- ❌ Resource usage reporting (CPU/RAM/disk) balik ke Panel

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
- [ ] `DockerEnvironment` — integrasi Docker SDK asli (butuh VPS)
- [ ] SFTP server (butuh VPS)
- [ ] WebSocket console real-time (butuh VPS)
- [ ] Resource usage reporting ke Panel (butuh VPS)

## License

MIT
