import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";
import path from "path";
import { componentTagger } from "lovable-tagger";

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => ({
  server: {
    host: "::",
    port: 8080,
    hmr: {
      overlay: false,
    },
    proxy: {
      "/api": {
        target: "http://localhost:3000",
        changeOrigin: true,
        // Suppress misleading proxy error logs for expected HTTP errors
        // (e.g. 401 on /auth/refresh when no session cookie exists).
        // Real connection errors (ECONNREFUSED, etc.) are still logged.
        configure: (proxy) => {
          proxy.on("error", (err, _req, res) => {
            // If the response has already been written (HTTP error from backend),
            // suppress the redundant Vite error log.
            if (res && (res as any).headersSent) return;
            console.error("[proxy error]", err.message);
          });
        },
      },
      "/uploads": {
        target: "http://localhost:3000",
        changeOrigin: true,
      },
    },
  },
  plugins: [react(), mode === "development" && componentTagger()].filter(Boolean),
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
    dedupe: ["react", "react-dom", "react/jsx-runtime", "react/jsx-dev-runtime", "@tanstack/react-query", "@tanstack/query-core"],
  },
}));
