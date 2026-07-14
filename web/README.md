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

The dashboard includes the bounded recent-event preview and the `Ctrl/Cmd+K` command surface. This is the sole gitsss web client. Production bundles are served by the Go binary from its adjacent `web/` directory; source runs use `web/dist`.
