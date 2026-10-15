export type GitsssReactPage = 'code' | 'issues' | 'pulls' | 'actions' | 'projects' | 'settings' | 'wiki' | 'releases' | '';

// gitsssReactPage picks the React view from the repository path.
// classic=1 keeps the server-rendered page.
export function gitsssReactPage(pathname: string, search: string): GitsssReactPage {
  if (new URLSearchParams(search).get('classic') === '1') return '';
  const parts = pathname.split('/').filter((part) => part !== '');
  if (parts.length < 2) return '';
  const rest = parts.slice(2);
  const head = rest[0] ?? '';
  if (head === '' || head === 'src' || head === 'tree' || head === 'find' || head === 'media') return 'code';
  if (head === 'issues') return 'issues';
  if (head === 'pulls') return 'pulls';
  if (head === 'actions') return 'actions';
  if (head === 'projects' && /^[0-9]+$/.test(rest[1] ?? '')) return 'projects';
  if (head === 'settings') return 'settings';
  if (head === 'wiki') return 'wiki';
  if (head === 'releases') return 'releases';
  return '';
}

export function projectID(pathname: string): string {
  const parts = pathname.split('/').filter((part) => part !== '');
  return parts[3] ?? '';
}
