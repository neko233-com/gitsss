import {tr} from './i18n.ts';

test('React views follow the rendered page locale', () => {
  expect(tr('企业公告', {}, 'zh-CN')).toBe('企业公告');
  expect(tr('企业公告', {}, 'en-US')).toBe('Organization announcements');
  expect(tr('企业公告', {}, 'fr-FR')).toBe('Organization announcements');
  expect(tr('{name} 的权限已保存', {name: 'Developers'}, 'en-US')).toBe('Developers permissions saved');
  expect(tr('{name} 的权限已保存', {name: '研发组'}, 'zh-CN')).toBe('研发组 的权限已保存');
});
