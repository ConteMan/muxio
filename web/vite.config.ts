import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// The build writes straight into the Go embed directory (ADR-007). There is no
// intermediate copy step, so the committed assets are exactly what Vite emits.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    outDir: "../internal/webui/assets",
    emptyOutDir: true,
  },
  server: {
    // `npm run dev` serves the UI while `muxio serve` provides the API.
    proxy: {
      "/api": "http://127.0.0.1:8080",
      "/readyz": "http://127.0.0.1:8080",
      "/healthz": "http://127.0.0.1:8080",
    },
  },
});
