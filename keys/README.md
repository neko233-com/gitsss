# gitsss release trust root

`release-ed25519.pub` is the out-of-band Ed25519 trust root for signed gitsss update manifests.

- Algorithm: Ed25519
- Key ID: `11d4f94bdcb5c016`
- Public key encoding: unpadded base64, one line

The private seed is not stored in this repository. Rotating this key requires a separately signed transition manifest and an explicit administrator trust update; replacing this file alone must not silently authorize a new release key.
