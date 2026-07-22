import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Tauri serves the built frontend via a custom protocol (tauri://localhost).
// Use a relative base so assets resolve regardless of the protocol origin, and
// a fixed port so the Tauri dev server can find Vite.
export default defineConfig({
  plugins: [react()],
  // Asset paths are relative so they work under tauri://localhost and the
  // production bundle's custom protocol.
  base: "./",
  clearScreen: false,
  server: {
    port: 1420,
    strictPort: true,
    watch: {
      // Don't watch the Rust side from Vite.
      ignored: ["**/src-tauri/**"],
    },
  },
});
