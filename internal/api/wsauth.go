package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type wsClaims struct {
	ServerUUID string `json:"server_uuid"`
	Exp        int64  `json:"exp"`
}

// verifyWSToken verifikasi JWT HS256 (cuma exp + server_uuid yang dipakai),
// signed pakai secret yang sama kayak auth_token Wings (= daemon_token Node
// di Panel). Ditulis manual biar nggak nambah dependency JWT di Go.
func verifyWSToken(token, secret string) (*wsClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("format token salah")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := mac.Sum(nil)

	got, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || subtle.ConstantTimeCompare(got, expected) != 1 {
		return nil, errors.New("signature nggak valid")
	}

	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("payload nggak bisa dibaca")
	}

	var claims wsClaims
	if err := json.Unmarshal(payloadRaw, &claims); err != nil {
		return nil, errors.New("payload nggak valid")
	}

	if time.Now().Unix() > claims.Exp {
		return nil, errors.New("token expired")
	}

	return &claims, nil
}
