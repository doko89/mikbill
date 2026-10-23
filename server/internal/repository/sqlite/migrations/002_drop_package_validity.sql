-- Mikbill tidak lagi mengatur durasi per paket. Masa aktif mengikuti
-- session-timeout profile hotspot di router.
ALTER TABLE packages DROP COLUMN validity_seconds;
