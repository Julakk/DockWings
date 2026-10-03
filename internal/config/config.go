package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config nyimpen semua setting daemon Wings.
// Di-load dari file config.json di root project (atau path yang di-set via flag -config).
type Config struct {
	// ListenAddr — alamat + port buat HTTP API Wings, ex: ":8080"
	ListenAddr string `json:"listen_addr"`

	// SFTPAddr — alamat + port buat SFTP server, ex: ":2022"
	SFTPAddr string `json:"sftp_addr"`

	// AuthToken — shared secret yang dipakai buat validasi request dari Panel.
	// Harus sama persis sama "daemon_token" yang di-generate Panel pas bikin Node.
	AuthToken string `json:"auth_token"`

	// DockerSocket — path ke docker socket, biasanya /var/run/docker.sock
	DockerSocket string `json:"docker_socket"`

	// DataDirectory — folder tempat nyimpen file server-server game
	DataDirectory string `json:"data_directory"`

	// BackupDirectory — folder arsip backup server (tar.gz). Jangan di dalam DataDirectory.
	BackupDirectory string `json:"backup_directory"`

	// SSL — TLS opsional buat HTTP API, mirip blok `api.ssl` di Wings Pterodactyl.
	// Kalau Enabled=true, Node di Panel harus pakai scheme https.
	// Kalau false (default), Node di Panel harus pakai scheme http.
	SSL SSLConfig `json:"ssl"`
}

// SSLConfig — setting TLS buat HTTP API Wings.
type SSLConfig struct {
	Enabled bool   `json:"enabled"`
	Cert    string `json:"cert"` // path fullchain.pem
	Key     string `json:"key"`  // path privkey.pem
}

// Default nilai konfigurasi kalau file config belum ada.
func Default() Config {
	return Config{
		ListenAddr:      ":8080",
		SFTPAddr:        ":2022",
		AuthToken:       "",
		DockerSocket:    "/var/run/docker.sock",
		DataDirectory:   "/var/lib/dockwings/servers",
		BackupDirectory: "/var/lib/dockwings/backups",
	}
}

// Load baca config dari path JSON. Kalau file belum ada, balikin Default().
func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("gagal baca config: %w", err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("config.json nggak valid: %w", err)
	}

	if cfg.AuthToken == "" {
		return cfg, fmt.Errorf("auth_token wajib diisi di config.json, dapetin dari Panel pas bikin Node")
	}

	if cfg.SSL.Enabled && (cfg.SSL.Cert == "" || cfg.SSL.Key == "") {
		return cfg, fmt.Errorf("ssl.enabled=true tapi ssl.cert / ssl.key belum diisi")
	}

	return cfg, nil
}
