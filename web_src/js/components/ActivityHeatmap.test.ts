import {createApp, h} from 'vue';
import ActivityHeatmap from './ActivityHeatmap.vue';

test('activity summary renders signed changes next to each value', () => {
  const today = new Date();
  const expired = new Date(today); expired.setDate(expired.getDate() - 365);
  const root = document.createElement('div');
  const app = createApp({render: () => h(ActivityHeatmap, {
    values: [{date: today, count: 5}, {date: expired, count: 9}],
    locale: {
      textTotalContributions: '', noDataText: '', tooltipUnit: 'events',
      heatMapLocale: {months: new Array(12).fill(''), days: new Array(7).fill(''), on: '', more: '', less: ''},
      labels: {change: 'Change since yesterday'},
    },
  })});
  app.mount(root);
  const changes = Array.from(root.querySelectorAll('.heatmap-summary-change'));
  expect(changes.map((node) => node.textContent)).toEqual(['(-4)', '(+5)', '(+5)', '(+0)', '(-4)']);
  expect(changes[0].getAttribute('aria-label')).toBe('Change since yesterday: -4');
  expect(root.querySelectorAll('.heatmap-summary dd')[3].textContent).toContain(' / 365');
  app.unmount();
});
