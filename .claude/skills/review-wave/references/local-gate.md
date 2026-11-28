# Local gate

Run checks for the packages the branch touched before the reviewers start and
after selected fixes. Then run the full host-platform test suite once on the
final tree before the PR. CI still runs the whole suite on three OSes; the
final host run catches failures in tests outside the directly changed packages.
It does not catch every platform-specific or flaky test.

## Packages

The touched packages are the folders of the changed `.go` files:

```sh
git diff --name-only origin/main...HEAD -- '*.go' | xargs -r -n1 dirname | sort -u | sed 's|^|./|'
```

Add a package whose tests exercise the changed code from outside, for
example `./cmd/snow` (the e2e tests) for a server or client change.

For documentation-only changes, check whitespace, changed links/references, skill
frontmatter, and instruction consistency. Report Go checks as not applicable when
no code, dependency, build, or lint configuration changes. Structural validation of
a skill does not establish that agents follow it correctly.

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

## Final pre-PR test

After the final code or test change, run the full suite once on the host:

```sh
# Linux or macOS
go test -race ./...

# Windows
go test ./...
```

Reuse a passing result if no Go source, test, fixture, dependency, or build
configuration changed afterward. Keep the final result in the review report
and the PR body. CI remains responsible for the other operating systems.

## Results

- A failure stops the gate. Fix it, commit, and run the gate again before
  the reviewers start or before the PR is opened.
- Put one line per check in the report's human part: the check, the OS, and
  pass or fail. Put the same lines in the PR body under "How it was tested".
