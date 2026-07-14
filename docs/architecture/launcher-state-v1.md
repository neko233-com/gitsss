# Launcher active state v1

`gitsss-launcher` is the stable process entrypoint required for self-updating `gitsss-agent` on Windows, macOS and Linux. A platform service keeps the launcher binary open; the launcher reads an immutable active-state generation and starts the selected versioned Agent binary. Updating the Agent therefore does not overwrite the executable currently held by the service manager.

State files use canonical JSON and names such as `active-00000000000000000001.json`. A transition always publishes a new, no-replace generation; it never edits or deletes the current generation in place. Rollback publishes a still-higher generation pointing to the previous version, preserving an audit-friendly sequence.

Each state binds:

- monotonically increasing generation;
- release version and exact 40-character commit;
- absolute version directory and working directory;
- the platform-native `gitsss-agent` executable name;
- the exact Agent argument vector, including only the protected Token file path rather than its value.

The launcher rejects non-canonical/unknown JSON, mismatched filename generation, relative or unclean paths, unsafe arguments, symlink executables and non-regular executables. It invokes the Agent directly without a shell, inherits stdout/stderr for platform logging, forwards shutdown, waits for a bounded grace period and kills only when the child does not stop.

Installed release packages auto-detect the sibling launcher and `BUILDINFO.json`. The autostart transaction creates the protected Token, immutable generation 1, and platform service definition before registering Task Scheduler, launchd or systemd; those services execute only `gitsss-launcher --state-dir ...`. Update activation still needs to publish a new generation, restart the platform service, run a health probe and publish a rollback generation if the probe fails.
