-- 003_allowlist: daftar host allowlist walled garden (mis. channel pembayaran).
CREATE TABLE IF NOT EXISTS walled_garden_allowlist (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  dst_host TEXT NOT NULL UNIQUE,
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
