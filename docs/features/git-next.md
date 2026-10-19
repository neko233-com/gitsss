# git-next child repository

Public source: https://github.com/neko233-com/git-next . The parent records only a gitlink. `.gitmodules` sets `ignore=all`, branch main and fetchRecurseSubmodules=false. Parent `.gitignore` also ignores the directory. Inspect deliberately with `git diff --ignore-submodules=none` and `git -C git-next status`. Child source, builds and dependencies are independent of GitSSS; production keeps its established Git/OpenSSH executable.

Current client is experimental; read the child's compatibility matrix. Its installer installs a separate git-next preview and keeps a git-named preview outside PATH. Full replacement remains blocked until complete compatibility is proven. Installation on one workstation does not authorize replacing Git on another machine. Source publication is separate from a consolidated binary release.

Use explicit scoped child commits and normal pushes before updating the parent pointer. No workflows or GitHub Actions. Parent releases must not bundle ignored child caches or imply the child has passed full Git conformance. Roll back only the gitlink for a parent dependency rollback; it does not modify production Git repositories, LFS objects, users or SSH keys.


## Repository terminal

The independent child also provides `git-terminal`: Windows native PowerShell with a Start menu entry and optional folder context menu; Unix uses the native shell. It preserves the existing Git command and Git Bash. Bash/POSIX emulation and complete Git client parity remain unaccepted; this terminal entry does not claim either.
