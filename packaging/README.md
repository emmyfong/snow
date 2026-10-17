# Releasing Snow

1. Tag the release on `main` and push the tag:

   ```sh
   git tag v0.1.0
   git push origin v0.1.0
   ```

   The release workflow builds the archives, the `.deb` packages,
   `checksums.txt`, and the Scoop and winget manifests, and publishes a
   GitHub release with all of them.

2. Scoop: download `snow.json` from the release and commit it as
   `bucket/snow.json` in the next pull request.

3. winget: download the three `EmmyFong.snow*.yaml` files from the release,
   check them with `winget validate --manifest <folder>`, and open a pull
   request to [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs)
   under `manifests/e/EmmyFong/snow/<version>/`.

Check a change to `.goreleaser.yaml` locally with
`goreleaser release --snapshot --clean`; it writes everything to `dist/`.
