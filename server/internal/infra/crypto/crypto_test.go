package crypto

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

const testKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestProtectorRoundtrip(t *testing.T) {
	p, err := NewProtector(testKey)
	if err != nil {
		t.Fatal(err)
	}
	for _, plain := range []string{"rahasia-router", "", "kata sandi cukup panjang 123!@#"} {
		enc, err := p.Seal(plain)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(enc, plain) && plain != "" {
			t.Fatalf("ciphertext membocorkan plaintext: %q", enc)
		}
		got, err := p.Open(enc)
		if err != nil {
			t.Fatal(err)
		}
		if got != plain {
			t.Fatalf("roundtrip gagal: got %q want %q", got, plain)
		}
	}
}

func TestProtectorNonceAcak(t *testing.T) {
	p, err := NewProtector(testKey)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := p.Seal("sama")
	b, _ := p.Seal("sama")
	if a == b {
		t.Fatal("nonce tidak acak: dua seal sama persis")
	}
}

func TestProtectorKunciSalah(t *testing.T) {
	for _, key := range []string{"", "xyz", strings.Repeat("ab", 16), strings.Repeat("zz", 32)} {
		if _, err := NewProtector(key); err == nil {
			t.Fatalf("kunci %q seharusnya ditolak", key)
		}
	}
	p, _ := NewProtector(testKey)
	if _, err := p.Open("bukan-base64!!!"); err == nil {
		t.Fatal("ciphertext rusak seharusnya gagal")
	}
	lain, _ := NewProtector(strings.Repeat("ff", 32))
	enc, _ := p.Seal("data")
	if _, err := lain.Open(enc); err == nil {
		t.Fatal("buka dengan kunci lain seharusnya gagal")
	}
}

func TestPasswordCost12(t *testing.T) {
	hash, err := HashPassword("katasandi123")
	if err != nil {
		t.Fatal(err)
	}
	if c, err := bcrypt.Cost([]byte(hash)); err != nil || c != 12 {
		t.Fatalf("cost bcrypt = %d, want 12", c)
	}
	if BcryptCost != 12 {
		t.Fatalf("BcryptCost = %d, want 12", BcryptCost)
	}
	if !CheckPassword(hash, "katasandi123") {
		t.Fatal("password benar ditolak")
	}
	if CheckPassword(hash, "salah") {
		t.Fatal("password salah diterima")
	}
}

func TestRandomString(t *testing.T) {
	s, err := RandomString(8)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 8 {
		t.Fatalf("panjang = %d, want 8", len(s))
	}
	for _, c := range s {
		if !strings.ContainsRune(VoucherAlphabet, c) {
			t.Fatalf("karakter %q di luar alfabet", c)
		}
	}
	for _, ambigu := range []string{"0", "O", "1", "I", "l"} {
		if strings.Contains(VoucherAlphabet, ambigu) {
			t.Fatalf("alfabet mengandung karakter ambigu %q", ambigu)
		}
	}
	lain, _ := RandomString(8)
	if s == lain {
		t.Fatal("dua string acak sama persis")
	}
}

func TestRandomToken(t *testing.T) {
	tok, err := RandomTokenURL(32)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil || len(raw) != 32 {
		t.Fatalf("token sesi tidak valid: %q", tok)
	}
	csrf, err := RandomTokenHex(32)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = hex.DecodeString(csrf)
	if err != nil || len(raw) != 32 {
		t.Fatalf("token CSRF tidak valid: %q", csrf)
	}
}
