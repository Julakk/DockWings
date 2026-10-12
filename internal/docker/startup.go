package docker

// startupShell menjalankan start command pakai bash kalau image punya,
// dan jatuh ke sh kalau tidak. Egg Pterodactyl sering memakai [[ ]] dan sintaks
// bash lain yang nggak dikenal dash. exec dipakai supaya proses utama tetap PID 1
// dan input console lewat /proc/1/fd/0 tetap sampai.
const startupShell = `if [ -x /bin/bash ]; then exec /bin/bash -c "$1"; else exec /bin/sh -c "$1"; fi`

// startupArgs mengembalikan argumen docker untuk image dan start command.
func startupArgs(image, startup string) []string {
	if startup == "" {
		return []string{image}
	}
	return []string{"--entrypoint", "/bin/sh", image, "-c", startupShell, "dockwings-startup", startup}
}
