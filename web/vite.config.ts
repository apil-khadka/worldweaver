import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { resolve } from "path";

export default defineConfig({
  root: ".",
  plugins: [react()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
    rollupOptions: {
      input: {
        main: resolve(__dirname, "index.html"),
        play: resolve(__dirname, "play.html"),
        landing: resolve(__dirname, "landing/index.html"),
      },
    },
  },
  server: {
    proxy: {
      "/ws": {
        target: "http://localhost:8080",
        ws: true,
      },
      "/api": {
        target: "http://localhost:8080",
      },
    },
  },
});
