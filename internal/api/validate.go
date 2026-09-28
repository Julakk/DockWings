package api

import "regexp"

var (
	uuidRe  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)
	imageRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/:@-]{0,254}$`)
)

func validUUID(s string) bool  { return uuidRe.MatchString(s) }
func validImage(s string) bool { return imageRe.MatchString(s) }
