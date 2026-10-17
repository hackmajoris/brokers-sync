import { defineConfig, lazyPlugins } from "vite-plus"
import react from "@vitejs/plugin-react"
import fs from "fs"
import path from "path"
import { mockApi } from "./mockApi"

// `vite build --mode watchlist` builds the standalone watchlist app into its
// own dist so its S3 bucket never holds the portfolio bundle.
export default defineConfig(({ mode }) => ({
  staged: {
    "*": "vp check --fix",
  },
  fmt: {
    semi: false,
  },
  lint: {
    jsPlugins: [{ name: "vite-plus", specifier: "vite-plus/oxlint-plugin" }],
    rules: { "vite-plus/prefer-vite-plus-imports": "error" },
    options: { typeAware: true, typeCheck: true },
  },
  define: {
    __BUILD_DATE__: JSON.stringify(new Date().toISOString()),
  },
  plugins: lazyPlugins(() => [
    react(),
    mockApi(),
    {
      name: "serve-portfolio-data",
      configureServer(server) {
        server.middlewares.use("/data/result.json", (_req, res) => {
          const filePath = path.resolve(__dirname, "../data/result.json")
          if (!fs.existsSync(filePath)) {
            res.statusCode = 404
            res.end()
            return
          }
          res.setHeader("Content-Type", "application/json")
          res.setHeader("Cache-Control", "no-cache")
          fs.createReadStream(filePath).pipe(res)
        })
      },
    },
  ]),
  server: {
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
  build:
    mode === "watchlist"
      ? {
          outDir: "dist-watchlist",
          rollupOptions: { input: path.resolve(__dirname, "watchlist.html") },
        }
      : undefined,
}))
