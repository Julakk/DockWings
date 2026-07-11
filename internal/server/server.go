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
	Status          Status            `json:"status"`
}

// ContainerName konsisten sama pola yang dipakai Panel (lihat App\Models\Server::containerName()).
func (s *Server) ContainerName() string {
	return "dockpanel-" + s.UUID
}
