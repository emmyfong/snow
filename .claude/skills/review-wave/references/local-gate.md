# Local gate

Run the same checks CI runs, on the packages the branch touched, before the
reviewers start and again after the fixes, before the PR. CI runs the whole
suite on three OSes; this gate catches most failures in a few minutes and
spares the user a red PR. It does not catch every flaky test.

## Packages

The touched packages are the folders of the changed `.go` files:

```sh
git diff --name-only origin/main...HEAD -- '*.go' | xargs -r -n1 dirname | sort -u | sed 's|^|./|'
```

Add a package whose tests exercise the changed code from outside, for
example `./cmd/snow` (the e2e tests) for a server or client change.

## Checks

Use the golangci-lint version that `.github/workflows/` pins.

```sh
go build ./...
for os in linux windows darwin; do GOOS=$os golangci-lint run ./...; done
go vet <packages>
go test -count=1 <packages>
go test -count=1 -race <packages>
```

On a Windows machine, or in WSL (which runs Windows programs), run the same
packages' tests on Windows. Build each package's test binary and run it from
the package folder, so tests find their `testdata`:

```sh
( cd <package> && GOOS=windows GOARCH=amd64 go test -c -o w.test.exe . \
  && ./w.test.exe; rm -f w.test.exe )
```

On Linux or macOS without Windows, run `GOOS=windows go vet <packages>`
instead and say in the report that Windows tests did not run.

## Results

- A failure stops the gate. Fix it, commit, and run the gate again before
  the reviewers start or before the PR is opened.
- Put one line per check in the report's human part: the check, the OS, and
  pass or fail. Put the same lines in the PR body under "How it was tested".
