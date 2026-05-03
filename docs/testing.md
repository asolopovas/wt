# Testing

stdlib `testing` only. Names: `Test<Function>_<Scenario>`, table-driven preferred. Config tests: `t.TempDir()` + `t.Setenv("HOME", ...)`.

CI runs `go vet`, `golangci-lint`, full `go test` on Linux.

Diarization integration: `//go:build integration` (`task test INTEGRATION=1`); `getSherpaSample` lazy-downloads to `samples/diarization/sherpa/` (gitignored). Use `-short` to skip download. Don't add skip-on-missing-model tests.

Never pure `go test ./internal/transcriber/...` from shell — whisper.cpp cgo bindings need prebuilt lib. Use `task test SHORT=1` (skips cgo for unrelated packages, still builds transcriber via prep step).

GUI compile-checks **only** through `task build ONLY=gui` — `CGO_LDFLAGS` differs between MinGW (`-lwhisper`) and CUDA/MSVC (`whisper.lib`).
