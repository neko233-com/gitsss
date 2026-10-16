# Public source hygiene

Installation-specific hostnames, internal service addresses, organization names
and credentials belong in private configuration, Actions variables/Secrets or the
deployed database. Public examples use neutral names and reserved example domains.
Never commit the private deny-list itself.

Before committing or publishing, run the local check with a private JSON file
outside the repository. It accepts a list of literal strings or a replacement
map (its keys are checked). Reports contain filenames/object IDs, never matched
values. The current-source scan includes tracked and non-ignored new files.

```sh
python3 tools/check-public-data.py --deny-file /private/public-deny.json
python3 tools/check-public-data.py --deny-file /private/public-deny.json --all-history
```

`task verify:public -- --deny-file /private/public-deny.json` runs the same check.
Inspect screenshots and packaged/embedded documentation too; a byte scan cannot
recognize text rendered inside an image or compressed archive.

Removing current files does not remove older Git objects. Historical cleanup needs
a verified private bundle, an isolated rewrite, before/after scans, `git fsck`,
explicit maintainer authorization and compare-and-swap updates for each affected
branch/tag. Keep the bundle private. Re-clone after a rewrite; merging or pushing
old branches can restore the removed history. Existing release binaries retain
their original contents and must be audited separately from rewritten source refs.

GitHub manages pull-request refs and cached commit/diff views separately. Rewriting
normal branches/tags cannot erase those read-only references or third-party clones.
If complete removal is required, the repository owner must request GitHub's
repository-data/cache cleanup and handle any forks. Never claim that rewriting
branches alone erases all public copies.
