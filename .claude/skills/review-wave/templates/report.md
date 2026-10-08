# Review: #<pr> <title>

Reviewed SHA: `<sha>`. Reviewers: <n> x <model>, curated by <main model>.

## For humans

### Problem

- <what the PR sets out to do, from the issue>
- <constraints that shape it>

### Verdict

- <ready to merge | merge after fixes | needs rework>, because <one line>.

### Findings

| # | Severity | Finding | Recommended fix |
|---|---|---|---|
| 1 | blocker | <short> | <short> |

### Proposed fixes and tradeoffs

- **1. <finding>.** <fix in one or two lines>. Alternative: <option>, which
  <tradeoff>.

```
<ASCII diagram: before and after for each blocker and each non-obvious fix>
```

### Glossary

- **<term>**: <definition>.

---

## For agents

### Artifact

- PR: #<pr>, head `<sha>`, base `<base>`.
- Linked issue: #<issue>.
- Previous report: <path and SHA, or "none">.

### Prior checklist (re-reviews only)

| Finding | Status at `<sha7>` | Evidence |
|---|---|---|
| <old finding> | fixed / open / regressed | <evidence> |

### Acceptance criteria

| # | Criterion | Status | Evidence |
|---|---|---|---|
| 1 | <criterion> | met / not met / cannot tell | <evidence> |

### Finding 1: <title>

- Severity: <severity>
- Location: `<file:line>`
- Symptom: <what goes wrong>
- Evidence: <quote, command, output>
- Impact: <who is affected and how badly>
- Required change: <exact change>
- Tests: <test to add or change, and what it proves>
- Alternatives: <option>: <tradeoff>.

### Verification run

- <commands the main reviewer ran and their results>
