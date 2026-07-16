# gitsss offline setup-go

This repository-local MIT implementation provides the common `actions/setup-go`
contract without downloading a toolchain. It accepts the moving `v4` through
`v6` references and their recorded moving-ref commit hashes. Go 1.26 is embedded
in the gitsss image; requests for another minor version fail closed and
`GOTOOLCHAIN=local` prevents implicit downloads.

Reference behavior was reviewed against the official
`https://github.com/actions/setup-go` documentation on 2026-07-16.
