# Testing

stdlib `testing` only. Names: `Test<Function>_<Scenario>`, table-driven preferred. Config tests: `t.TempDir()` + `t.Setenv("HOME", ...)`.

CI runs `go vet`, `golangci-lint`, full `go test` on Linux.

Diarization integration: `//go:build integration` (`task test INTEGRATION=1`); `getSherpaSample` lazy-downloads to `samples/diarization/sherpa/` (gitignored). Use `-short` to skip download. Don't add skip-on-missing-model tests.

Use `task test` (or `task test SHORT=1` to skip cgo-heavy packages). Bare `go test ./internal/transcriber/...` from a shell can break depending on local env (sherpa-onnx headers, CUDA flags); the Task wrappers set the right `CGO_*` flags.

GUI compile-checks **only** through `task build ONLY=gui` — `CGO_LDFLAGS` differs between MinGW and CUDA/MSVC.
