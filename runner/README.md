# Embedded gitsss runner

The runner is maintained as an internal gitsss module. It is compiled into the
single `gitsss/gitsss` product image and is not published as a separate binary,
image, or command.

The image entrypoint registers the internal runner against the local gitsss
server after `/api/healthz` succeeds. Common Actions and their runtimes are
stored under `actions/builtin`; unknown remote Actions fail closed and are not
downloaded from the internet.

Runner development uses the root project formatting, linting, test, and image
verification targets. Production operators manage runners through the gitsss
administration UI and persist runner state in the single `/data` volume.
