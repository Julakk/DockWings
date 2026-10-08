package docker

import "context"

// imageLocal true kalau image sudah ada di host (mis. dibangun lokal dan
// nggak ada di registry), jadi pull yang gagal nggak perlu menggagalkan create.
func imageLocal(ctx context.Context, image string) bool {
	_, err := run(ctx, "image", "inspect", image)
	return err == nil
}
