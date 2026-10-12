package main

import (
	"flag"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/Julakk/DockWings/internal/api"
	"github.com/Julakk/DockWings/internal/config"
	"github.com/Julakk/DockWings/internal/docker"
	"github.com/Julakk/DockWings/internal/server"
	"github.com/Julakk/DockWings/internal/sftpd"
	"github.com/Julakk/DockWings/internal/version"
)

func main() {
	configPath := flag.String("config", "config.json", "path ke file config JSON")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("gagal load config: %v", err)
	}

	mgr := server.NewPersistentManager("/var/lib/dockwings/servers.json")

	// TODO: ganti ke DockerEnvironment asli begitu development lanjut
	// ke tahap integrasi Docker (butuh VPS buat testing).
	env := docker.NewDockerEnvironment(cfg.DataDirectory)
	env.SetUser(cfg.ContainerUID, cfg.ContainerGID)
	go env.RunCrashWatchdog()

	router := api.NewRouter(
		mgr, env, cfg.AuthToken,
		api.WithFilesRoot(cfg.DataDirectory),
		api.WithBackups(cfg.DataDirectory, cfg.BackupDirectory),
	)

	if cfg.PanelURL != "" && cfg.SFTPAddr != "" {
		keyPath := filepath.Join(
			filepath.Dir(cfg.DataDirectory),
			"sftp_host_ed25519_key",
		)
		hk, hkErr := sftpd.LoadOrCreateHostKey(keyPath)
		if hkErr != nil {
			log.Printf("SFTP nonaktif: %v", hkErr)
		} else {
			sf := sftpd.New(sftpd.Config{
				DataRoot: cfg.DataDirectory,
				HostKey:  hk,
				Auth:     sftpd.NewPanelAuth(cfg.PanelURL, cfg.AuthToken),
			})
			go func() {
				if err := sf.ListenAndServe(cfg.SFTPAddr); err != nil {
					log.Printf("SFTP berhenti: %v", err)
				}
			}()
			log.Printf("SFTP jalan di %s", cfg.SFTPAddr)
		}
	} else {
		log.Printf("SFTP nonaktif (isi panel_url di config.json)")
	}

	scheme := "http"
	if cfg.SSL.Enabled {
		scheme = "https"
	} else {
		log.Printf("PERINGATAN: ssl.enabled=false.")
		log.Printf("auth_token lewat jaringan tanpa enkripsi.")
		log.Printf("Aktifkan ssl atau pasang reverse proxy HTTPS.")
	}
	log.Printf("DockWings v%s jalan di %s://%s", version.Version, scheme, cfg.ListenAddr)

	// Timeout cuma di header + idle, bukan Read/WriteTimeout, supaya
	// upload/download besar dan WebSocket console tetap normal.
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 16,
	}

	var serveErr error
	if cfg.SSL.Enabled {
		serveErr = srv.ListenAndServeTLS(cfg.SSL.Cert, cfg.SSL.Key)
	} else {
		serveErr = srv.ListenAndServe()
	}
	if serveErr != nil {
		log.Fatalf("server berhenti: %v", serveErr)
	}
}
