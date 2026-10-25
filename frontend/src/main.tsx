import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import "@fontsource/outfit/400.css";
import "@fontsource/outfit/500.css";
import "@fontsource/outfit/600.css";
import "@fontsource/outfit/700.css";
import "@fontsource/outfit/800.css";
import "./index.css";
import App from "./App.tsx";
import { useTheme } from "./store/theme";
import { applyTheme } from "./lib/theme";

// Muat tema publik: cache diterapkan sinkron agar tanpa flash,
// lalu disegarkan dari server.
void useTheme.getState().load();

// Terapkan ulang aksen saat preferensi sistem berubah dalam mode system.
if (typeof window !== "undefined" && typeof window.matchMedia === "function") {
  const mq = window.matchMedia("(prefers-color-scheme: dark)");
  const ikutSistem = () => {
    const s = useTheme.getState();
    if (s.theme_mode === "system") applyTheme(s);
  };
  if (typeof mq.addEventListener === "function") mq.addEventListener("change", ikutSistem);
  else mq.addListener(ikutSistem);
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </StrictMode>,
);
