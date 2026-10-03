package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/Julakk/DockWings/internal/api"
	"github.com/Julakk/DockWings/internal/config"
	"github.com/Julakk/DockWings/internal/docker"
	"github.com/Julakk/DockWings/internal/server"
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

	router := api.NewRouter(mgr, env, cfg.AuthToken, api.WithFilesRoot(cfg.DataDirectory), api.WithBackups(cfg.DataDirectory, cfg.BackupDirectory))

	scheme := "http"
	if cfg.SSL.Enabled {
		scheme = "https"
	}
	log.Printf("DockWings v%s jalan di %s://%s (Docker environment)", version.Version, scheme, cfg.ListenAddr)

	var serveErr error
	if cfg.SSL.Enabled {
		serveErr = http.ListenAndServeTLS(cfg.ListenAddr, cfg.SSL.Cert, cfg.SSL.Key, router)
	} else {
		serveErr = http.ListenAndServe(cfg.ListenAddr, router)
	}
	if serveErr != nil {
		log.Fatalf("server berhenti: %v", serveErr)
	}
}
