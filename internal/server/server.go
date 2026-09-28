package server

// Status merepresentasiin state server game di daemon ini.
type Status string

const (
	StatusInstalling Status = "installing"
	StatusOffline    Status = "offline"
	StatusStarting   Status = "starting"
	StatusRunning    Status = "running"
	StatusStopping   Status = "stopping"
)

// Server nyimpen data satu instance server game yang dikelola Wings.
// Ini versi ringkas dari model `Server` di sisi Panel — cuma field yang
// dibutuhin buat provisioning & kontrol container.
// Allocation adalah satu pasangan ip:port yang di-mapping 1:1 ke container
// (host port == container port, ngikutin konvensi Pterodactyl: port di-inject
// ke container lewat env SERVER_PORT, bukan lewat remapping port Docker).
type Allocation struct {
	IP      string `json:"ip"`
	Port    int    `json:"port"`
	Primary bool   `json:"primary"`
}

type Server struct {
	UUID            string            `json:"uuid"`
	Image           string            `json:"image"`
	StartupCommand  string            `json:"startup_command"`
	MemoryLimitMB   int64             `json:"memory_limit"`
	SwapMB          int64             `json:"swap"`
	IOWeight        int64             `json:"io_weight"`
	CPULimitPercent float64           `json:"cpu_limit"`
	DiskSpaceMB     int64             `json:"disk_space"`
	EnvVariables    map[string]string `json:"env_variables"`
	Allocations     []Allocation      `json:"allocations"`
	Status          Status            `json:"status"`
}

// PrimaryAllocation balikin allocation primary, atau allocation pertama
// kalau nggak ada yang ditandai primary, atau nil kalau kosong.
func (s *Server) PrimaryAllocation() *Allocation {
	if len(s.Allocations) == 0 {
		return nil
	}
	for i := range s.Allocations {
		if s.Allocations[i].Primary {
			return &s.Allocations[i]
		}
	}
	return &s.Allocations[0]
}

// ContainerName konsisten sama pola yang dipakai Panel (lihat App\Models\Server::containerName()).
func (s *Server) ContainerName() string {
	return "dockpanel-" + s.UUID
}
