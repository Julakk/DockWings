package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/Julakk/DockWings/internal/api"
	"github.com/Julakk/DockWings/internal/config"
	"github.com/Julakk/DockWings/internal/docker"
	"github.com/Julakk/DockWings/internal/server"
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
	env := docker.NewDockerEnvironment("/var/lib/dockwings/servers")

	router := api.NewRouter(mgr, env, cfg.AuthToken)

	log.Printf("DockWings jalan di %s (Docker environment)", cfg.ListenAddr)

	if err := http.ListenAndServe(cfg.ListenAddr, router); err != nil {
		log.Fatalf("server berhenti: %v", err)
	}
}
