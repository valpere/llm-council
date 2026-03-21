---
name: ship
description: Implement a GitHub issue end-to-end — select issue, branch, code, tests, PR, review, merge. Without args: shows open issues to select from. This is the entry point for processing any existing GitHub issue.
user-invocable: true
argument-hint: "[issue-number]"
metadata:
  version: "2.0"
  author: backend-claude
  last_updated: "2026-03-21"
---

# /ship

Implement a GitHub issue from selection to merged PR.

```
/ship → select issue → branch → implement → pre-flight → PR → Copilot → /fix-review → merge
```

## Rules

- Only ship PRs created by Claude or explicitly named by the user. Never touch Dependabot PRs.
- Branch protection is on — no direct pushes to `main`. Always go through a PR.
- One round of Copilot comments only. After `/fix-review`, do not wait for re-review.
- If Copilot has no comments (or only approves), merge immediately.
- After merge: `git checkout main && git pull`.
- **Always wait for user confirmation before starting implementation.**

---

## Step 0: Select issue

If called with no argument, list open GitHub issues sorted by priority:

```bash
gh issue list --repo valpere/llm-council-backend --state open \
  --json number,title,labels \
  --jq 'sort_by(.labels[].name) | .[] | "#\(.number) \(.title) [\([.labels[].name] | join(", "))]"'
```

Display as a numbered menu. Wait for user to select.

If called with an issue number (e.g. `/ship 29`), skip the menu.

---

## Step 1: Read the issue

```bash
gh issue view <number> --repo valpere/llm-council-backend --json title,body,labels
```

Read the `## Summary` and `## Acceptance Criteria` sections. These define what done looks like.

---

## Step 2: Read affected files

Read every file that will change before writing anything. Do not guess.

Typical candidates:
- **API / HTTP** — `internal/api/handler.go`
- **Council logic** — `internal/council/council.go`, `interfaces.go`, `types.go`, `prompts.go`
- **Config** — `internal/config/config.go`
- **Storage** — `internal/storage/storage.go`
- **Entry point** — `cmd/server/main.go`
- **Tests** — `internal/council/council_test.go`, `internal/api/handler_test.go`

Run `go build ./...` to confirm baseline compiles.

---

## Step 3: Present implementation approach and wait for confirmation

Briefly describe:
- What files will change and why
- Debt level (⚡/⚖️/🏗️)
- Any design decisions with more than one reasonable answer

**Stop here. Do not write code until the user confirms.**

---

## Step 4: Create branch and implement

```bash
git checkout -b <type>/<slug>   # e.g. test/handler-tests, fix/cors-origins
```

Implement changes:
- Stay within layer boundaries (no business logic in handlers, no net/http in council/storage)
- Follow existing patterns in the codebase
- Write tests at the declared debt level

Layer boundaries (enforce strictly):
```
cmd/server/main.go     ← wiring only
internal/api/          ← parse → call interfaces → respond; no logic
internal/council/      ← deliberation logic; no net/http
internal/storage/      ← persistence; no net/http, no council
internal/openrouter/   ← LLM calls; no council, no storage
```

---

## Step 5: Pre-flight

```bash
go build ./...
go vet ./...
go test ./...
git status
git log main..HEAD --oneline
```

Fix any failures from your changes before proceeding. Note pre-existing failures separately.

---

## Step 6: Create PR

```bash
gh pr create \
  --title "<debt-emoji> <type>(<scope>): <description>" \
  --body "$(cat <<'EOF'
## Summary
<bullet points>

Closes #<issue-number>

## Test plan
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

Debt emoji in title: `⚡` quick-fix · `⚖️` balanced · `🏗️` proper-refactor

---

## Step 7: Wait for Copilot

```bash
gh pr checks <number> --watch
```

Wait up to 5 minutes. If no review appears, proceed to merge.

---

## Step 8: Address comments

Run `/fix-review <number>` — one round only. Push fixes. Do not wait for re-review.

---

## Step 9: Merge

```bash
gh pr merge <number> --squash --delete-branch
git checkout main && git pull
```

---

## Step 10: Report

Summarise: issue closed, PR number, what Copilot flagged (if anything), merge commit.

## What NOT to do

- Do not bump version numbers or update changelogs unless asked.
- Do not open follow-up issues unless review reveals a real bug outside the PR scope.
- Do not run `go mod tidy` unless the PR actually adds/removes dependencies.
