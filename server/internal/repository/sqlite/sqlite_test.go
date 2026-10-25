package sqlite

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mikbil/mikbil/internal/domain"
)

func openMigrated(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMigrateSemuaTabel(t *testing.T) {
	db := openMigrated(t)
	want := []string{
		"admins", "settings", "mikrotik_connections", "packages",
		"orders", "vouchers", "webhook_deliveries", "audit_logs",
		"sessions", "walled_garden_allowlist", "theme_assets", "schema_migrations",
	}
	for _, table := range want {
		var name string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name)
		if err != nil {
			t.Fatalf("tabel %s hilang: %v", table, err)
		}
	}
	rows, err := db.Query(`SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal("versi migrasi tidak tercatat:", err)
	}
	var versions []string
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			t.Fatal(err)
		}
		versions = append(versions, version)
	}
	rows.Close()
	if len(versions) != 4 || versions[0] != "001_init.sql" ||
		versions[1] != "002_drop_package_validity.sql" || versions[2] != "003_allowlist.sql" ||
		versions[3] != "004_theme.sql" {
		t.Fatalf("versi = %v, want 001 + 002 + 003 + 004", versions)
	}
	cols, err := db.Query(`SELECT name FROM pragma_table_info('packages')`)
	if err != nil {
		t.Fatal(err)
	}
	for cols.Next() {
		var name string
		if err := cols.Scan(&name); err != nil {
			t.Fatal(err)
		}
		if name == "validity_seconds" {
			t.Fatal("kolom validity_seconds masih ada setelah migrasi 002")
		}
	}
	cols.Close()
	// Migrasi ulang harus idempotent.
	if err := Migrate(db); err != nil {
		t.Fatal("migrasi ulang gagal:", err)
	}
}

func TestAdminRepo(t *testing.T) {
	db := openMigrated(t)
	r := NewAdminRepo(db)
	a, err := r.Create(domain.Admin{Name: "Owner", Username: "owner", PasswordHash: "hash"})
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == 0 || a.CreatedAt.IsZero() {
		t.Fatalf("admin tersimpan tidak lengkap: %+v", a)
	}
	if _, err := r.Create(domain.Admin{Name: "X", Username: "owner", PasswordHash: "h"}); !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("duplikat username = %v, want ErrAlreadyExists", err)
	}
	got, err := r.GetByUsername("owner")
	if err != nil || got.Name != "Owner" {
		t.Fatalf("GetByUsername = %+v, %v", got, err)
	}
	if _, err := r.GetByUsername("tidak-ada"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("username asing = %v, want ErrNotFound", err)
	}
	n, err := r.Count()
	if err != nil || n != 1 {
		t.Fatalf("Count = %d, %v", n, err)
	}
}

func TestSettingsRepo(t *testing.T) {
	db := openMigrated(t)
	r := NewSettingsRepo(db)
	if _, found, _ := r.Get("kosong"); found {
		t.Fatal("kunci kosong seharusnya tidak found")
	}
	if err := r.Set("site_url", "https://wifi.contoh.com"); err != nil {
		t.Fatal(err)
	}
	v, found, err := r.Get("site_url")
	if err != nil || !found || v != "https://wifi.contoh.com" {
		t.Fatalf("Get = %q, %v, %v", v, found, err)
	}
	if err := r.Set("site_url", "https://baru.id"); err != nil {
		t.Fatal(err)
	}
	v, _, _ = r.Get("site_url")
	if v != "https://baru.id" {
		t.Fatalf("upsert gagal: %q", v)
	}
}

func TestThemeRepo(t *testing.T) {
	db := openMigrated(t)
	r := NewThemeRepo(db)
	if _, found, _ := r.Get(domain.ThemeLogoKey); found {
		t.Fatal("logo seharusnya belum ada")
	}
	data := []byte{0x89, 'P', 'N', 'G'}
	if err := r.Set(domain.ThemeLogoKey, "image/png", data); err != nil {
		t.Fatal(err)
	}
	a, found, err := r.Get(domain.ThemeLogoKey)
	if err != nil || !found || a.MIME != "image/png" || string(a.Data) != string(data) {
		t.Fatalf("Get = %+v, %v, %v", a, found, err)
	}
	if err := r.Delete(domain.ThemeLogoKey); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := r.Get(domain.ThemeLogoKey); found {
		t.Fatal("logo masih ada setelah hapus")
	}
}

func TestConnectionRepo(t *testing.T) {
	db := openMigrated(t)
	r := NewConnectionRepo(db)
	if _, err := r.GetActive(); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("tanpa koneksi = %v, want ErrNotFound", err)
	}
	c, err := r.CreateActive(domain.Connection{
		Name: "default", Host: "192.168.88.1", Port: 8728,
		Username: "admin", EncPassword: "enc", UseTLS: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !c.IsActive {
		t.Fatal("koneksi baru harus aktif")
	}
	got, err := r.GetActive()
	if err != nil || got.Host != "192.168.88.1" {
		t.Fatalf("GetActive = %+v, %v", got, err)
	}
	if _, err := r.CreateActive(domain.Connection{Host: "10.0.0.1", Port: 8729, Username: "a", EncPassword: "e"}); err != nil {
		t.Fatal(err)
	}
	got, _ = r.GetActive()
	if got.Host != "10.0.0.1" {
		t.Fatalf("koneksi aktif = %q, want 10.0.0.1", got.Host)
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM mikrotik_connections WHERE is_active = 1`).Scan(&n)
	if n != 1 {
		t.Fatalf("koneksi aktif = %d, want 1", n)
	}
}

func TestSessionRepo(t *testing.T) {
	db := openMigrated(t)
	admins := NewAdminRepo(db)
	a, _ := admins.Create(domain.Admin{Name: "O", Username: "o", PasswordHash: "h"})
	r := NewSessionRepo(db)
	s := domain.Session{ID: "hash1", AdminID: a.ID, CSRFToken: "csrf", ExpiresAt: time.Now().Add(time.Hour)}
	if err := r.Create(s); err != nil {
		t.Fatal(err)
	}
	got, err := r.Get("hash1")
	if err != nil || got.AdminID != a.ID || got.CSRFToken != "csrf" {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if _, err := r.Get("asing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("sesi asing = %v, want ErrNotFound", err)
	}
	if err := r.Delete("hash1"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Get("hash1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("sesi terhapus masih terbaca")
	}
}
