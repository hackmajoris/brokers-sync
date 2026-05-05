import type { TooltipItem } from 'chart.js';
import { useChart } from './useChart';
import { chartDefaults } from '../../lib/chartDefaults';

interface BarChartProps {
  data: Array<Record<string, number | string>>
  keyX: string
  keyY: string
  color?: string
  colorFn?: (v: number) => string
  height?: number
  formatValue?: (v: number) => string
}

export function BarChart({ data, keyX, keyY, color = '#fb923c', colorFn, height = 160, formatValue }: BarChartProps) {
  const labels = data.map(d => String(d[keyX]));
  const values = data.map(d => d[keyY] as number);

  const bgColors = colorFn ? values.map(v => colorFn(v) + 'aa') : color + 'aa';
  const bdColors = colorFn ? values.map(v => colorFn(v)) : color;

  const ref = useChart({
    type: 'bar',
    data: {
      labels,
      datasets: [{
        data: values,
        backgroundColor: bgColors,
        borderColor: bdColors,
        borderWidth: 1,
        borderRadius: 4,
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
            label: (ctx: TooltipItem<'bar'>) => {
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
    <div style={{ position: 'relative', width: '100%', height }}>
      <canvas ref={ref} />
    </div>
  );
}
