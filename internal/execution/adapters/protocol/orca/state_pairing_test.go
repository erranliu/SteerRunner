package orca

import (
	"encoding/base64"
	"testing"
)

func TestStatePairingHostID(t *testing.T) {
	for _, tc := range []struct {
		name, key, want string
	}{
		{"zero-key", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", "sha256:66687aadf862bd776c8fc18b8e9f8e20089714856ee233b3902a591d0d5f2925"},
		{"sequence-key", "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=", "sha256:630dcd2966c4336691125448bbb25b4ff412a49c732db2c8abc1b8581bd710dd"},
		{"crlf", "AAECAwQFBgcICQoL\r\nDA0ODxAREhMUFRYXGBkaGxwdHh8=\r\n", "sha256:630dcd2966c4336691125448bbb25b4ff412a49c732db2c8abc1b8581bd710dd"},
		// Key length remains the resolver's responsibility. Keep the legacy
		// decoder behavior, including accepting nonzero trailing padding bits.
		{"short-key", "YWJj", "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		{"empty-key", "", "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"padding-bits", "YR==", "sha256:ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := (statePairing{PublicKey: tc.key}).hostID()
			if err != nil || got != tc.want {
				t.Fatal(got, err)
			}
		})
	}
	for _, tc := range []struct {
		key  string
		want base64.CorruptInputError
	}{
		{"!", 0},
		{"AA=A", 2},
		{"YWJj!", 4},
		{"AAAA ", 4},
	} {
		t.Run(tc.key, func(t *testing.T) {
			got, err := (statePairing{PublicKey: tc.key}).hostID()
			if got != "" || err != tc.want {
				t.Fatal(got, err)
			}
		})
	}
}
