import {createApp, h} from 'vue';
import ActivityHeatmap from './ActivityHeatmap.vue';
import {localUserSettings} from '../modules/user-settings.ts';

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

test('saved default settings migrate while custom five-level settings remain intact', () => {
  const oldColors = ['#183b60', '#245b88', '#317eb4', '#55a9df', '#a5dfff'];
  const oldBounds = [1, 5, 10, 20, 50];
  const render = () => {
    const root = document.createElement('div');
    const app = createApp({render: () => h(ActivityHeatmap, {
      values: [{date: new Date(), count: 1}],
      locale: {
        textTotalContributions: '', noDataText: '', tooltipUnit: 'events',
        heatMapLocale: {months: new Array(12).fill(''), days: new Array(7).fill(''), on: '', more: '', less: ''},
        labels: {change: 'Change'},
      },
    })});
    app.mount(root);
    return {root, app};
  };
  try {
    localUserSettings.setString('gitsss:heatmap-display:v1', JSON.stringify({bounds: oldBounds, colors: oldColors}));
    const migrated = render();
    expect(migrated.root.querySelectorAll('.heatmap-legend > span')).toHaveLength(5);
    expect(migrated.root.querySelector('.heatmap-day[data-count="1"]')!.getAttribute('style')).toContain('var(--heatmap-level-1)');
    migrated.app.unmount();

    localUserSettings.setString('gitsss:heatmap-display:v1', JSON.stringify({bounds: oldBounds, colors: ['#123456', ...oldColors.slice(1)]}));
    const custom = render();
    expect(custom.root.querySelectorAll('.heatmap-legend > span')).toHaveLength(6);
    expect(custom.root.querySelector('.heatmap-day[data-count="1"]')!.getAttribute('style')).toContain('#123456');
    custom.app.unmount();
  } finally {
    localUserSettings.setString('gitsss:heatmap-display:v1', 'null');
  }
});
