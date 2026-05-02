# LLM Council — Code Review

The `RoleBasedReview` pipeline runs four specialised reviewer roles in parallel and
asks a chairman to consolidate their findings. Use it via `POST /api/conversations/{id}/review`.

---

## How it works

```
Code diff (content)
    │
    ▼
┌──────────────────────────────────────────────────────────┐
│ Stage 1 — Parallel role execution                        │
│                                                          │
│  security      →  OWASP Top 10, injection, secrets, …   │
│  logic         →  nil dereferences, race conditions, …   │
│  simplicity    →  DRY/KISS/YAGNI, naming, duplication    │
│  architecture  →  SOLID violations, layer boundaries, …  │
│                                                          │
│  Each role returns a JSON array of findings              │
└─────────────────────────┬────────────────────────────────┘
                           │
                           ▼
                    Stage 2 — skipped
                    (roles are complementary, not competing)
                           │
                           ▼
┌──────────────────────────────────────────────────────────┐
│ Stage 3 — Chairman synthesis                             │
│  Reads all four role outputs → consolidated review       │
└──────────────────────────────────────────────────────────┘
```

All four roles must succeed. If any role call fails, the pipeline returns `503 council
quorum not met` (or an SSE `error` event on the streaming path).

---

## The four roles

| Role | Focus |
|------|-------|
| `security` | OWASP Top 10, auth/authz flaws, input validation, SQL/command injection, hardcoded secrets, cryptography misuse, unsafe API usage |
| `logic` | Edge cases, nil/null pointer dereferences, off-by-one errors, race conditions, incorrect error propagation, missing bounds checks |
| `simplicity` | DRY violations, overly complex logic (KISS), premature abstraction (YAGNI), poor naming, missing or misleading comments |
| `architecture` | Layer boundary violations, dependency direction, tight coupling, low cohesion, interface design, SOLID principle violations |

Each role is instructed to return **only** a JSON array of findings:

```json
[
  {
    "file": "internal/api/handler.go",
    "line": 42,
    "severity": "high",
    "body": "User-controlled input passed to os.Exec without sanitisation."
  }
]
```

An empty array (`[]`) means no findings for that role.

---

## Configuration

```bash
# .env — code review models (optional, defaults to COUNCIL_MODELS)

# One model per role recommended; fewer are assigned round-robin.
CODE_REVIEW_MODELS=openai/gpt-4o-mini,anthropic/claude-haiku-4-5,google/gemini-flash-1.5,openai/gpt-4o-mini

# Chairman model for synthesis (optional, defaults to CHAIRMAN_MODEL)
CODE_REVIEW_CHAIRMAN_MODEL=anthropic/claude-sonnet-4-5
```

**Model assignment:** `models[i % len(models)]`

- 1 model → all 4 roles use it
- 2 models → security+simplicity get model[0], logic+architecture get model[1]
- 4 models → one model per role (recommended for best coverage)

> Provider diversity matters: different model families catch different classes of bugs.

---

## Usage

### Synchronous (blocking)

```bash
CONV_ID=$(curl -s -X POST http://localhost:8001/api/conversations | jq -r .id)

curl -s -X POST http://localhost:8001/api/conversations/$CONV_ID/review \
  -H "Content-Type: application/json" \
  -d "{\"content\": \"$(git diff -U8 | jq -Rs .)\"}" \
  | jq .stage3.content
```

### Streaming (SSE)

```bash
curl -N http://localhost:8001/api/conversations/$CONV_ID/review/stream \
  -X POST \
  -H "Content-Type: application/json" \
  -d "{\"content\": \"$(git diff -U8 | jq -Rs .)\"}"
```

Events arrive in order:

```
data: {"type":"stage1_complete","data":[{"label":"security","content":"[...]",...},
                                        {"label":"logic","content":"[]",...},
                                        {"label":"simplicity","content":"[...]",...},
                                        {"label":"architecture","content":"[]",...}]}

data: {"type":"stage2_complete","data":[],"metadata":{"council_type":"code-review",
       "label_to_model":{...},"aggregate_rankings":[],"consensus_w":1.0}}

data: {"type":"stage3_complete","data":{"content":"## Code Review Summary\n\n...","model":"...","duration_ms":2100}}

data: {"type":"title_complete","data":{"title":"Code Review Summary"}}

data: {"type":"complete"}
```

### Recommended diff format

```bash
# Current uncommitted changes (8 lines of context for better analysis)
git diff -U8

# Specific commit
git show <sha> -U8

# PR diff
git diff main...feature/my-branch -U8
```

---

## Response structure

`stage3.content` is a markdown document produced by the chairman. Typical structure:

```markdown
## Code Review Summary

### Critical Issues

**[security]** `cmd/server/main.go:42` (high)
User-controlled input passed to os.Exec without sanitisation.

### Warnings

**[simplicity]** `internal/api/handler.go:88` (low)
Function `handleRequest` is 120 lines — consider splitting by concern.

### No Issues Found

- **logic** — no logical errors detected
- **architecture** — no structural problems detected

### Overall Assessment

The diff is safe to merge after addressing the one critical security finding.
```

---

## Raw role output

`stage1_complete` contains each role's raw JSON findings array. Useful for
programmatic processing before the chairman's narrative.

```bash
# Extract all high/critical findings across all roles
curl -s ... | jq '
  .stage1
  | map(.content | fromjson)
  | flatten
  | map(select(.severity == "critical" or .severity == "high"))
'
```

---

## Error handling

| Condition | HTTP | SSE |
|-----------|------|-----|
| Empty `content` field | `400 Bad Request` | — |
| Invalid conversation UUID | `400 Bad Request` | — |
| Conversation not found | `404 Not Found` | — |
| Any role call fails (quorum=4, all required) | `503 Service Unavailable` | `{"type":"error","message":"council quorum not met"}` |
| Chairman call fails | `500 Internal Server Error` | `{"type":"error","message":"internal server error"}` |

---

## See also

- [`docs/api.md`](api.md) — full HTTP API reference including SSE event shapes
- [`docs/pipeline.md`](pipeline.md) — code-level walkthrough of both pipelines
- [`internal/council/review_roles.go`](../internal/council/review_roles.go) — role definitions
- [`internal/council/rolebased.go`](../internal/council/rolebased.go) — pipeline implementation
