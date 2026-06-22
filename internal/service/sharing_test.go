package service

import (
	"testing"

	"github.com/exndiver/shopping-backend/internal/models"
)

func TestNewInviteToken_urlSafeAndUnique(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 1000; i++ {
		tok, err := newInviteToken()
		if err != nil {
			t.Fatalf("newInviteToken: %v", err)
		}
		if len(tok) != 22 { // 16 bytes -> 22 base64url chars (no padding)
			t.Fatalf("unexpected token length %d for %q", len(tok), tok)
		}
		for _, c := range tok {
			ok := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
				(c >= '0' && c <= '9') || c == '-' || c == '_'
			if !ok {
				t.Fatalf("token %q contains non-url-safe char %q", tok, c)
			}
		}
		if _, dup := seen[tok]; dup {
			t.Fatalf("duplicate token generated: %q", tok)
		}
		seen[tok] = struct{}{}
	}
}

func TestShareAccess_Valid(t *testing.T) {
	cases := map[models.ShareAccess]bool{
		models.ShareAccessView: true,
		models.ShareAccessEdit: true,
		"":                     false,
		"owner":                false,
		"VIEW":                 false,
	}
	for access, want := range cases {
		if got := access.Valid(); got != want {
			t.Errorf("ShareAccess(%q).Valid() = %v, want %v", access, got, want)
		}
	}
}
