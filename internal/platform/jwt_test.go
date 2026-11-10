package platform

import (
	"crypto/ed25519"
	"testing"
	"time"
)

func TestSignAndVerify(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	otherPub, _, _ := ed25519.GenerateKey(nil)

	good, _ := SignToken(priv, "user-1", "user", time.Minute)
	expired, _ := SignToken(priv, "user-1", "user", -time.Minute)
	tampered := good[:len(good)-3] + "AAA"

	tests := []struct {
		name  string
		token string
		pub   ed25519.PublicKey
		ok    bool
	}{
		{"ispravan token", good, pub, true},
		{"istekao token", expired, pub, false},
		{"tuđ javni ključ", good, otherPub, false},
		{"izmenjen token", tampered, pub, false},
		{"smeće", "nije.token.uopste", pub, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			claims, err := VerifyToken(tc.pub, tc.token)
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v, očekivano ok=%v", err, tc.ok)
			}
			if tc.ok && (claims.Subject != "user-1" || claims.Role != "user") {
				t.Errorf("pogrešni claim-ovi: %+v", claims)
			}
		})
	}
}
