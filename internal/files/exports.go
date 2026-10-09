package files

// Resolve, ResolveNoFollow, dan IsRootPath dipakai server SFTP supaya aturan
// pengurungan folder server-nya sama persis dengan file manager.
func (s *Store) Resolve(rel string) (string, error) { return s.resolve(rel) }

func (s *Store) ResolveNoFollow(rel string) (string, error) { return s.resolveNoFollow(rel) }

// Base balikin folder server (path host). Cuma buat pengecekan keberadaan.
func (s *Store) Base() string { return s.base }

func IsRootPath(rel string) bool { return isRoot(rel) }
