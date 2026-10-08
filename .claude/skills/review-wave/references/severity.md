# Severity scale

- **blocker**: Merging makes shipped behavior wrong, loses data, breaks a
  supported OS, or breaks CI. Example: a PTY reader goroutine writes to the
  Bubble Tea model directly and races.
- **major**: Correct today, but a likely next change breaks, or an acceptance
  criterion is only partly met. Example: an error is wrapped without `%w`, so
  callers cannot match the typed error.
- **minor**: Quality problem with a small, local effect. Example: a test
  covers the success path but not the error path of a new function.
- **nit**: Style or naming the linters do not catch. Example: a doc comment
  that restates the function name.
