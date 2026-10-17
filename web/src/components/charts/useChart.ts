import { useEffect, useRef } from "react"
import { Chart, type ChartConfiguration, type ChartType } from "chart.js"

export function useChart<T extends ChartType>(config: ChartConfiguration<T>) {
  const ref = useRef<HTMLCanvasElement>(null)
  const chartRef = useRef<Chart<T> | null>(null)

  useEffect(() => {
    if (!ref.current) return
    if (chartRef.current) chartRef.current.destroy()
    const ctx = ref.current.getContext("2d")
    if (!ctx) return
    chartRef.current = new Chart(ctx, config)
    return () => {
      if (chartRef.current) {
        chartRef.current.destroy()
        chartRef.current = null
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [JSON.stringify(config)])

  return ref
}
