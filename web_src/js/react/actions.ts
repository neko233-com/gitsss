export type BoardCard = {id: number; number: number; title: string};

export function moveIssuesPayload(cards: BoardCard[]): {issues: {issueID: number; sorting: number}[]} {
  return {
    issues: cards.map((card, index) => ({issueID: card.id, sorting: index})),
  };
}

export function fileUpdatePayload(content: string, sha: string, message: string, branch: string): {content: string; sha: string; message: string; branch: string} {
  return {content: utf8ToBase64(content), sha, message, branch};
}

export function utf8ToBase64(value: string): string {
  const bytes = new TextEncoder().encode(value);
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}

export function repoSettingsPayload(input: {description: string; private: boolean; hasWiki: boolean; defaultBranch: string}): {description: string; private: boolean; has_wiki: boolean; default_branch: string} {
  return {
    description: input.description,
    private: input.private,
    has_wiki: input.hasWiki,
    default_branch: input.defaultBranch,
  };
}

export function collaboratorPayload(permission: string): {permission: string} {
  return {permission};
}

export function webhookPayload(target: string): {type: string; config: {url: string; content_type: string}; events: string[]; active: boolean} {
  return {
    type: 'gitea',
    config: {url: target, content_type: 'json'},
    events: ['push'],
    active: true,
  };
}

export function mergePayload(): {Do: string} {
  return {Do: 'merge'};
}

export function commentPayload(body: string): {body: string} {
  return {body};
}
