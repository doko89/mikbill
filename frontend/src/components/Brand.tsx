import { useState } from "react";
import { WifiHigh } from "@phosphor-icons/react";
import { useTheme } from "../store/theme";

// Brand menampilkan logo + nama + tagline layanan dari tema publik.
export default function Brand() {
  const brand_name = useTheme((s) => s.brand_name);
  const brand_tagline = useTheme((s) => s.brand_tagline);
  const logo_url = useTheme((s) => s.logo_url);
  const [gagalUrl, setGagalUrl] = useState<string | null>(null);
  const gagal = logo_url !== null && logo_url === gagalUrl;

  return (
    <div className="flex min-w-0 items-center gap-3">
      {logo_url && !gagal ? (
        <img
          key={logo_url}
          src={logo_url}
          alt={brand_name}
          className="h-11 w-11 shrink-0 rounded object-contain"
          onError={() => setGagalUrl(logo_url)}
        />
      ) : (
        <span
          className="flex h-11 w-11 shrink-0 items-center justify-center rounded-[10px]"
          style={{ background: "var(--accent)", color: "var(--accent-ink)" }}
        >
          <WifiHigh size={24} weight="bold" />
        </span>
      )}
      <div className="min-w-0">
        <h1 className="truncate text-xl font-bold tracking-tight">{brand_name}</h1>
        {brand_tagline ? (
          <p className="truncate text-sm" style={{ color: "var(--text-2)" }}>
            {brand_tagline}
          </p>
        ) : null}
      </div>
    </div>
  );
}
