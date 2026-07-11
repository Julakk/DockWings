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
}

// Default nilai konfigurasi kalau file config belum ada.
func Default() Config {
	return Config{
		ListenAddr:    ":8080",
		SFTPAddr:      ":2022",
		AuthToken:     "",
		DockerSocket:  "/var/run/docker.sock",
		DataDirectory: "/var/lib/dockwings/servers",
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

	return cfg, nil
}
