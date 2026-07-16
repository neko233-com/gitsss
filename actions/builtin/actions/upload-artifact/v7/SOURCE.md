# actions/upload-artifact v7 source

- Repository: `https://github.com/actions/upload-artifact`
- Moving major tag: `v7`
- Commit: `043fb46d1a93c77aae656e7c1c64a875d1fc6a0a`
- License: MIT, retained in `LICENSE`

The runtime bundle contains the published root and merge action manifests and
`dist/` files. Source, tests, and CI metadata are intentionally excluded.
gitsss maps the supported v4-v7 references to this immutable Node.js 24 copy.
The bundled runtime accepts any server that provides an authenticated Actions
Results endpoint instead of rejecting non-GitHub hostnames as GHES.
