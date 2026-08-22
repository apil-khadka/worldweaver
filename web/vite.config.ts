import { defineConfig } from "vite";

export default defineConfig({
  root: ".",
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/ws": {
        target: "http://localhost:8080",
        ws: true,
      },
    },
  },
});
