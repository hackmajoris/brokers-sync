import "./index.css"
import { StrictMode } from "react"
import { createRoot } from "react-dom/client"
import { WatchlistApp } from "./WatchlistApp"

const el = document.getElementById("root")!

createRoot(el).render(
  <StrictMode>
    <WatchlistApp />
  </StrictMode>,
)
