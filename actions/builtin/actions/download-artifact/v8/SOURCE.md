# actions/download-artifact v8 source

- Repository: `https://github.com/actions/download-artifact`
- Moving major tag: `v8`
- Commit: `3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c`
- License: MIT, retained in `LICENSE`

The runtime bundle contains the published action manifest and `dist/` files.
Source, tests, and CI metadata are intentionally excluded. gitsss maps the
supported v4-v8 references to this immutable, Node.js 24 runtime copy.
The bundled runtime accepts any server that provides an authenticated Actions
Results endpoint instead of rejecting non-GitHub hostnames as GHES.
