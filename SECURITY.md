# GPG Key Rotation

A typical lifetime of a GPG key is 2 years. Once it expires, the GitHub Action responsible for releasing the provider will show the error like `gpg: signing failed: Unusable secret key`.

---

## ✅ Steps to rotate the key

### 1. Generate a new key

```bash
gpg --full-generate-key
```

Recommended:
- Type: RSA and RSA
- Key size: 4096 bits
- Expiration: 2y
- Usage: Signing + Certify
- Name/email: match GitHub account or maintainer identity

Then list it:
```bash
gpg --list-secret-keys --keyid-format LONG
```

Copy the key ID (e.g., 845CE18D) and fingerprint.

```bash
gpg --armor --export-secret-keys YOUR_KEY_ID > private.asc
```

Export public key:

```bash
gpg --armor --export YOUR_KEY_ID > public.asc
```

Store both in 1Password (and optionally an encrypted USB or dotfiles repo you trust).

### 2. Update GitHub

Add the public key to [github](https://github.com/settings/keys). It will associate the key with your account.

Visit [actions](https://github.com/singlestore-labs/terraform-provider-singlestoredb/settings/secrets/actions) and update the variables `GPG_PRIVATE_KEY` and `PASSPHRASE`. This will update the GitHub Action responsible for the release to use the new key.

### 3. Update the Terraform Registry

[Sign in](https://registry.terraform.io/sign-in/legacy) to the Terraform Registry, visit the [GPG keys](https://registry.terraform.io/settings/gpg-keys) page, and click "New GPG Key". The namespace is "singlestore-labs". In case the namespace is not listed, reach out to the internal infra team. It's enough to fill in the "ASCII Armor" field with the public key and click "Save".

To test that everything works perform a new release, following [RELEASING.md](RELEASING.md).
