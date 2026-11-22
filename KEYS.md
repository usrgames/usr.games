# Keys

Every release of a usr.games program is signed, so that `tty1` and the installer can refuse one that is not. This page says what signs a release, how to check one yourself, and what happens when a key changes.

## The two keys

**The root key** stays with the studio and offline. It signs one thing only: the certificate of the release key. Its public half is `site/etc/allowed_signers`, served at `usr.games/etc/allowed_signers`. Its fingerprint:

```
SHA256:ftIfXrwCCHVZunpdo7y85JGaI1Gl1TdoP4RDicQhXSI
```

**The release key** signs each release's manifest, the list of a release's files with their hashes. It lives in the `release` environment of each repository that releases, where a tag `v*` and a person's approval are needed before a run can use it. The root vouches for it in a certificate, `site/etc/release-cert.pub`: for the identity `release@usr.games`, for one year.

So a player holds one thing, the root's public key, and it does not change when the release key does.

## Check a release by hand

A stock `ssh-keygen` does it, with the list from this site. Here `MANIFEST` stands for a release's manifest and `MANIFEST.sig` for its signature:

```sh
curl -fsSLO https://usr.games/etc/allowed_signers
ssh-keygen -Y verify -f allowed_signers -I release@usr.games -n release@usr.games -s MANIFEST.sig < MANIFEST
```

It answers `Good "release@usr.games" signature` or it refuses. Then check each file you downloaded against its hash in the manifest.

A signature is refused if the file changed, if it was made for any other purpose than a release, if its key has no certificate from the root, or if that certificate has ended.

## Each year: renew

The certificate is inside every signature, so a release stops checking out on the day its certificate ends. A test in this repository starts failing a month before that day. Then:

1. With the root key, sign a new certificate for the release key, with the next serial number:

   ```sh
   ssh-keygen -s root -I "usr.games release YYYY-MM" -n release@usr.games -V +52w -z N -O clear release.pub
   ```

2. Replace `site/etc/release-cert.pub` with it, in a pull request. The test checks that the root signed it.
3. In each repository that releases, sign the latest release again, so that it carries the new certificate. A program released since the renewal needs nothing.

## Replace the release key

Do it at once if the key may have been seen by anyone, and whenever a renewal is a good moment for it.

1. Make a new key: `ssh-keygen -t ed25519 -N "" -C "usr.games release" -f release`.
2. Sign its certificate with the root, as above.
3. Put the new key in the `release` environment of each repository, as the secret `RELEASE_KEY`.
4. Replace `site/etc/release-cert.pub`, and sign each latest release again.

Nothing a player holds changes.

## If a key is lost or taken

| What happened | What it means |
|---|---|
| The release key is lost | Nothing is at risk. Replace it. |
| The release key is taken | Replace it at once. Until the old certificate ends, a signature made with the old key still checks out: there is no list of revoked keys yet. |
| The root key is lost | No certificate can be signed again. A new root is needed, and every `tty1` has to be replaced by one that knows it. |
| The root key is taken | The same, and urgently. |
