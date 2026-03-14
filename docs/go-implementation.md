# Go Implementation Notes

The original Python/FastAPI + React implementation has been rewritten in Go. The frontend remains React; only the backend changed.

## Package Structure

```
llm-council/
├── cmd/
│   └── server/
│       └── main.go          # Entry point, server startup
├── internal/
│   ├── config/
│   │   └── config.go        # Config struct, Load() from env
│   ├── openrouter/
│   │   └── client.go        # QueryModel(), QueryModelsParallel()
│   ├── council/
│   │   ├── council.go       # RunFull(), stage functions, CalculateAggregateRankings()
│   │   └── types.go         # StageOneResult, StageTwoResult, etc.
│   ├── storage/
│   │   └── storage.go       # Create/Get/AddMessage/UpdateTitle/List
│   └── api/
│       └── handler.go       # HTTP handlers, CORS middleware, SSE streaming
├── frontend/                # Unchanged React app
├── data/
│   └── conversations/       # JSON conversation files
├── go.mod
├── go.sum
└── .env
```

## Key Implementation Notes

### Concurrency

Parallel model queries use `sync.WaitGroup` with per-goroutine results:

```go
results := make([]ModelResult, len(models))
var wg sync.WaitGroup
for i, model := range models {
    wg.Add(1)
    go func(i int, model string) {
        defer wg.Done()
        resp, err := client.QueryModel(ctx, model, messages, timeout)
        results[i] = ModelResult{Model: model, Response: resp, Err: err}
    }(i, model)
}
wg.Wait()
```

### SSE Streaming

Stage completion events are sent over a `text/event-stream` response. Each event is a single `data:` line containing a JSON object with a `type` field:

```
data: {"type":"stage1_start"}

data: {"type":"stage1_complete","data":[...]}

data: {"type":"stage2_complete","data":[...],"metadata":{"label_to_model":{...},"aggregate_rankings":[...]}}

data: {"type":"stage3_complete","data":{...}}

data: {"type":"title_complete","data":{"title":"..."}}

data: {"type":"complete"}
```

### Configuration

Loaded from environment variables (`.env` via `godotenv`):

```go
type Config struct {
    OpenRouterAPIKey string
    CouncilModels    []string
    ChairmanModel    string
    DataDir          string
    Port             string   // used as ":"+Port for http.ListenAndServe
}
```

### Storage

Each conversation is a single JSON file at `data/conversations/{uuid}.json`.

- Writes are atomic: data is written to a `.tmp` file then renamed, preventing partial writes on crash.
- Concurrent writes to the same conversation are serialized via a per-conversation `sync.Mutex`.
- Conversation IDs are validated against a UUID regex before any file path is constructed, preventing directory traversal.

### Error Handling

- Model query failures are logged; the caller skips failed results
- If all models fail in Stage 1, a descriptive error response is returned to the user
- If some models fail, the pipeline continues with successful responses
- Title generation failure is non-fatal; falls back to "New Conversation"
- Storage errors in the streaming path are logged (the SSE response has already started, so headers cannot be changed)

## Dependencies

| Package | Purpose |
|---------|---------|
| `net/http` | HTTP server (stdlib) |
| `encoding/json` | JSON encode/decode (stdlib) |
| `sync` | WaitGroup + per-conversation Mutex (stdlib) |
| `math/rand` | Label shuffle for Stage 2 anonymization (stdlib) |
| `github.com/google/uuid` | Conversation ID generation |
| `github.com/joho/godotenv` | Load `.env` file |

Standard library covers HTTP, JSON, concurrency, and file I/O. External dependencies are minimal.

## CORS

Allow `http://localhost:5173` (Vite dev server) and `http://localhost:3000` during development.

## Running

```bash
make dev                     # go run ./cmd/server
make build && ./bin/llm-council  # compiled binary
```

Frontend:
```bash
cd frontend && npm run dev
```
