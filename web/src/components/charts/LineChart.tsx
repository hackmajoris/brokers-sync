import type { TooltipItem } from 'chart.js';
import { useChart } from './useChart';
import { chartDefaults } from '../../lib/chartDefaults';

interface LineChartProps {
  data: Array<Record<string, number | string>>
  keyX: string
  keyY: string
  color?: string
  height?: number
  formatValue?: (v: number) => string
}

export function LineChart({ data, keyX, keyY, color = '#fb923c', height = 160, formatValue }: LineChartProps) {
  const labels = data.map(d => String(d[keyX]));
  const values = data.map(d => d[keyY] as number);

  const ref = useChart({
    type: 'line',
    data: {
      labels,
      datasets: [{
        data: values,
        borderColor: color,
        backgroundColor: color + '22',
        fill: true,
        borderWidth: 2,
        pointRadius: 3,
        pointHoverRadius: 5,
        pointBackgroundColor: color,
        pointBorderColor: '#0f0f0f',
        pointBorderWidth: 1.5,
        tension: 0.35,
      }],
    },
    options: {
      ...chartDefaults,
      interaction: { mode: 'index', intersect: false },
      plugins: {
        ...chartDefaults.plugins,
        tooltip: {
          ...(chartDefaults.plugins as Record<string, unknown>)['tooltip'] as object,
          callbacks: {
            label: (ctx: TooltipItem<'line'>) => {
              const v = ctx.parsed.y ?? 0;
              return formatValue ? formatValue(v) : `$${Math.round(v).toLocaleString()}`;
            },
          },
        },
      },
      scales: {
        x: {
          ...chartDefaults.scales?.['x'],
        },
        y: {
          ...chartDefaults.scales?.['y'],
          ticks: {
            ...(chartDefaults.scales?.['y'] as Record<string, unknown>)?.['ticks'] as object,
            callback: (v: number | string) => {
              const n = Number(v);
              if (formatValue) return formatValue(n);
              return n >= 1000 ? '$' + (n / 1000).toFixed(0) + 'k' : '$' + n;
            },
          },
        },
      },
    },
  });

  return (
    <div style={{ position: 'relative', height }}>
      <canvas ref={ref} />
    </div>
  );
}
