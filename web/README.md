# gitsss web

The React 19 frontend for gitsss, built with Vite 8, TypeScript 7, Tailwind CSS 4 and shadcn/ui Radix components.

## Commands

```sh
pnpm dev
pnpm lint
pnpm typecheck
pnpm test
pnpm build
```

The initial dashboard includes the bounded recent-event preview and the `Ctrl/Cmd+K` command surface. It is intentionally independent from the legacy Vue frontend so it can become the sole web client before `web_src/` is removed.
