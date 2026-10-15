import {GET} from '../modules/fetch.ts';

export type FrontendPluginSlot = 'dashboard' | 'repo-header';

export type FrontendPlugin = {
  id: string;
  src: string;
  slots: FrontendPluginSlot[];
};

const idPattern = /^[a-z0-9][a-z0-9-]{0,63}$/;
const srcPattern = /^\/assets\/plugins\/[a-z0-9][a-z0-9-]{0,63}\/[A-Za-z0-9._/-]+\.js$/;

type PluginModule = {
  mount?: (root: HTMLElement) => void | Promise<void>;
};

export function pluginScriptPath(src: string): string {
  if (src.includes('..') || src.includes('\\') || src.includes('?') || src.includes('#') || src.includes('://') || src.startsWith('//')) {
    throw new Error(`plugin src is not allowed: ${src}`);
  }
  if (!srcPattern.test(src)) {
    throw new Error(`plugin src must stay under /assets/plugins/<id>/*.js: ${src}`);
  }
  return src;
}

export function withAppSubUrl(path: string, appSubUrl: string): string {
  let prefix = appSubUrl;
  if (prefix.endsWith('/')) prefix = prefix.slice(0, -1);
  return `${prefix}${path}`;
}

export function parsePluginManifest(payload: unknown): FrontendPlugin[] {
  if (typeof payload !== 'object' || payload === null) {
    throw new Error('plugin manifest must be an object');
  }
  const plugins = (payload as {plugins?: unknown}).plugins;
  if (!Array.isArray(plugins)) {
    throw new Error('plugin manifest plugins must be an array');
  }
  return plugins.map((entry) => parsePlugin(entry));
}

function parsePlugin(entry: unknown): FrontendPlugin {
  if (typeof entry !== 'object' || entry === null) {
    throw new Error('plugin entry must be an object');
  }
  const record = entry as {id?: unknown; src?: unknown; slots?: unknown};
  if (typeof record.id !== 'string' || !idPattern.test(record.id)) {
    throw new Error('plugin id must be a lowercase slug');
  }
  if (typeof record.src !== 'string') {
    throw new Error(`plugin ${record.id} is missing src`);
  }
  pluginScriptPath(record.src);
  if (!Array.isArray(record.slots) || record.slots.length === 0) {
    throw new Error(`plugin ${record.id} needs at least one slot`);
  }
  const slots: FrontendPluginSlot[] = [];
  for (const slot of record.slots) {
    if (slot !== 'dashboard' && slot !== 'repo-header') {
      throw new Error(`plugin ${record.id} has unknown slot ${String(slot)}`);
    }
    slots.push(slot);
  }
  return {id: record.id, src: record.src, slots};
}

export async function loadFrontendPlugins(doc: Document, appSubUrl: string): Promise<void> {
  const slots = doc.querySelectorAll<HTMLElement>('[data-gitsss-plugin-slot]');
  if (slots.length === 0) return;
  const response = await GET(withAppSubUrl('/assets/plugins/manifest.json', appSubUrl), {
    cache: 'no-store',
    credentials: 'same-origin',
    headers: {accept: 'application/json'},
  });
  if (response.status === 404) return;
  if (!response.ok) {
    throw new Error(`plugin manifest returned ${response.status}`);
  }
  const plugins = parsePluginManifest(await response.json());
  for (const plugin of plugins) {
    const imported = await import(/* @vite-ignore */ withAppSubUrl(pluginScriptPath(plugin.src), appSubUrl)) as PluginModule;
    if (typeof imported.mount !== 'function') {
      throw new Error(`plugin ${plugin.id} does not export mount`);
    }
    for (const slot of slots) {
      const name = slot.getAttribute('data-gitsss-plugin-slot');
      if (name !== 'dashboard' && name !== 'repo-header') continue;
      if (!plugin.slots.includes(name)) continue;
      const host = doc.createElement('div');
      host.setAttribute('data-plugin-id', plugin.id);
      slot.append(host);
      await imported.mount(host);
    }
  }
}

export async function initFrontendPlugins(): Promise<void> {
  if (!document.querySelector('[data-gitsss-plugin-slot]')) return;
  try {
    await loadFrontendPlugins(document, window.config.appSubUrl);
  } catch (error) {
    console.error('frontend plugin host failed', error);
  }
}
