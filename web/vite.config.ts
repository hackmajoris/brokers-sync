import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import fs from 'fs'
import path from 'path'

export default defineConfig({
  plugins: [
    react(),
    {
      name: 'serve-portfolio-data',
      configureServer(server) {
        server.middlewares.use('/data/result.json', (_req, res) => {
          const filePath = path.resolve(__dirname, '../data/result.json')
          res.setHeader('Content-Type', 'application/json')
          res.setHeader('Cache-Control', 'no-cache')
          fs.createReadStream(filePath).pipe(res)
        })
      },
    },
  ],
})
