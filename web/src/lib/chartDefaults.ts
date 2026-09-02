import {
  Chart,
  type ChartOptions,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  Filler,
  Tooltip,
  Legend,
  LineController,
  BarController,
  DoughnutController,
} from 'chart.js';

Chart.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  Filler,
  Tooltip,
  Legend,
  LineController,
  BarController,
  DoughnutController,
);

export const C = {
  green:  '#34d399',
  red:    '#f87171',
  blue:   '#60a5fa',
  purple: '#a78bfa',
  orange: '#fb923c',
  yellow: '#fbbf24',
};

export const chartDefaults: ChartOptions = {
  plugins: {
    legend: { display: false },
    tooltip: {
      backgroundColor: '#1a1a2e',
      titleColor: '#eeeef8',
      bodyColor: 'rgba(238,238,248,0.7)',
      borderColor: 'rgba(255,255,255,0.1)',
      borderWidth: 1,
      padding: 10,
      cornerRadius: 8,
      displayColors: false,
    },
  },
  scales: {
    x: {
      grid: { color: 'rgba(255,255,255,0.04)' },
      ticks: {
        color: 'rgba(238,238,248,0.55)',
        font: { size: 10, family: 'DM Sans' },
        maxRotation: 0,
        minRotation: 0,
      },
      border: { display: false },
    },
    y: {
      grid: { color: 'rgba(255,255,255,0.04)' },
      ticks: {
        color: 'rgba(238,238,248,0.55)',
        font: { size: 10, family: 'DM Mono' },
        maxTicksLimit: 5,
      },
      border: { display: false },
    },
  },
  animation: { duration: 350 },
  maintainAspectRatio: false,
  responsive: true,
};
