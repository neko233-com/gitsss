# gitsss offline setup-node

This repository-local MIT implementation provides the common `actions/setup-node`
contract without downloading a toolchain. It accepts the moving `v4` through
`v7` references and their recorded moving-ref commit hashes. Node.js 24 is
embedded in the gitsss image; requests for another major version fail closed.

Reference behavior was reviewed against the official
`https://github.com/actions/setup-node` documentation on 2026-07-16.
