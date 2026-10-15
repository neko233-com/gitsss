import {parsePluginManifest, pluginScriptPath, withAppSubUrl} from './frontend-plugins.ts';

test('parsePluginManifest accepts a same-origin script', () => {
  const plugins = parsePluginManifest({
    plugins: [{id: 'hello-board', src: '/assets/plugins/hello-board/index.js', slots: ['dashboard']}],
  });
  expect(plugins).toEqual([
    {id: 'hello-board', src: '/assets/plugins/hello-board/index.js', slots: ['dashboard']},
  ]);
  expect(withAppSubUrl(pluginScriptPath(plugins[0].src), '/git')).toEqual('/git/assets/plugins/hello-board/index.js');
});

test('parsePluginManifest rejects remote and path escape', () => {
  expect(() => pluginScriptPath('https://evil.example/plugin.js')).toThrow(/not allowed/);
  expect(() => pluginScriptPath('/assets/plugins/hello/../../secret.js')).toThrow(/not allowed/);
  expect(() => parsePluginManifest({plugins: [{id: 'Hello', src: '/assets/plugins/hello/index.js', slots: ['dashboard']}]}))
    .toThrow(/lowercase slug/);
  expect(() => parsePluginManifest({plugins: [{id: 'hello', src: '/assets/plugins/hello/index.js', slots: ['admin']}]}))
    .toThrow(/unknown slot/);
});
