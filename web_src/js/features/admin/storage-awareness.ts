import * as echarts from 'echarts/core';
import {PieChart} from 'echarts/charts';
import {AriaComponent, LegendComponent, TooltipComponent} from 'echarts/components';
import {CanvasRenderer} from 'echarts/renderers';

echarts.use([AriaComponent, CanvasRenderer, LegendComponent, PieChart, TooltipComponent]);

function formatBytes(bytes: number) {
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB'];
  let unit = 0;
  while (bytes >= 1024 && unit < units.length - 1) {
    bytes /= 1024;
    unit++;
  }
  return `${bytes >= 10 || unit === 0 ? bytes.toFixed(0) : bytes.toFixed(1)} ${units[unit]}`;
}

function themeColor(name: string, fallback: string) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback;
}

export function initAdminStorageAwareness() {
  const element = document.querySelector<HTMLElement>('#storage-capacity-chart');
  if (!element) return;

  const usedBytes = Number(element.getAttribute('data-used-bytes'));
  const capacityBytes = Number(element.getAttribute('data-capacity-bytes'));
  if (!Number.isFinite(usedBytes) || !Number.isFinite(capacityBytes) || capacityBytes <= 0) return;

  const used = Math.min(Math.max(usedBytes, 0), capacityBytes);
  const available = capacityBytes - used;
  const chart = echarts.init(element);
  const usedLabel = element.getAttribute('data-label-used')!;
  const availableLabel = element.getAttribute('data-label-available')!;
  const textColor = themeColor('--color-text', '#17233a');
  const mutedTextColor = themeColor('--color-text-light-2', '#73829a');

  chart.setOption({
    aria: {enabled: true},
    color: [themeColor('--color-primary', '#2563eb'), themeColor('--color-green', '#22c55e')],
    legend: {
      bottom: 0,
      icon: 'circle',
      itemHeight: 8,
      itemWidth: 8,
      textStyle: {color: mutedTextColor},
    },
    series: [{
      type: 'pie',
      radius: ['62%', '82%'],
      center: ['50%', '42%'],
      avoidLabelOverlap: true,
      label: {
        color: textColor,
        formatter: `{ratio|${Math.round(used / capacityBytes * 100)}%}\n{label|${usedLabel}}`,
        rich: {
          ratio: {color: textColor, fontSize: 24, fontWeight: 600, lineHeight: 30},
          label: {color: mutedTextColor, fontSize: 12, lineHeight: 18},
        },
        position: 'center',
      },
      labelLine: {show: false},
      data: [
        {name: usedLabel, value: used},
        {name: availableLabel, value: available},
      ],
    }],
    tooltip: {
      trigger: 'item',
      valueFormatter: (value: number | string) => formatBytes(Number(value)),
    },
  });

  const observer = new ResizeObserver(() => chart.resize());
  observer.observe(element);
  window.addEventListener('pagehide', () => {
    observer.disconnect();
    chart.dispose();
  }, {once: true});
}
