## 1. Package scaffold

- [ ] 1.1 Create `internal/server/` directory with empty `server.go`, `handlers.go`, `browser.go`, `embed.go`, `server_test.go` (each declaring `package server`). Done when `go build ./internal/server` exits 0.
- [ ] 1.2 Create `internal/server/web/` containing `index.html` (one `<h1>Ezida viewer</h1>`), empty `app.js`, empty `style.css`. Done when `ls internal/server/web/` lists 3 files.
- [ ] 1.3 In `internal/server/embed.go`, declare `//go:embed web` over an `embed.FS` named `webFS`. Done when `go build ./internal/server` exits 0.

## 2. Server core

- [ ] 2.1 In `server.go`, define `type Options struct { Port int; NoOpen bool; Board string }` and a `Run(opts Options) error` entry point. Done when the signature compiles and a no-op test invokes it without panic (we'll fill the body next).
- [ ] 2.2 Implement port fallback per design (`net.Listen` loop over an 11-port window starting at `opts.Port`, default `7777`). Done when a unit test (`TestRun_PortFallback`) starts the server with a pre-bound listener on `7777` and observes the server choosing `7778`.
- [ ] 2.3 Add `PortUnavailableError` type in `internal/commands/errors.go` (or wherever the existing structured errors live; check `internal/output` first) with fields `Start int`, `Window int`. Done when `output.Fail` surfaces the code `PORT_UNAVAILABLE` in JSON mode against an instance of this error.
- [ ] 2.4 Wire `signal.NotifyContext(SIGINT, SIGTERM)` and call `srv.Shutdown(ctx, 5*time.Second)` on signal. Done when `TestRun_GracefulShutdown` starts the server in a goroutine, sends SIGINT to itself, and observes the goroutine returning within 1 s.

## 3. Handlers

- [ ] 3.1 In `handlers.go`, mount `GET /` via `http.HandlerFunc` reading `web/index.html` from `webFS`. Set `Content-Type: text/html; charset=utf-8`. Done when `TestHandle_Index` issues `GET /` against a test server and asserts status 200 + the embedded bytes.
- [ ] 3.2 Mount `GET /static/*` via `http.FileServerFS(fs.Sub(webFS, "web"))`. Done when `TestHandle_Static_App` issues `GET /static/app.js` and asserts the body matches the embedded stub.
- [ ] 3.3 Mount `GET /api/board` calling `board.Load(s.boardPath)` and encoding the response payload per design (`schema_version`, `columns`, `priorities`, `cards_per_column`, `cards`). Set `Content-Type: application/json`. Done when `TestHandle_Board_Valid` asserts the shape against a fixture board.
- [ ] 3.4 Implement an `httpError(w, err)` helper that maps `*board.SchemaVersionError` → `SCHEMA_VERSION_MISMATCH` (500), `*board.ValidationError` → `VALIDATION_FAILED` (500), `fs.ErrNotExist` → `BOARD_NOT_FOUND` (500), and other errors → `IO_ERROR` (500). Body shape matches ADR 0001 §D8. Done when `TestHandle_Board_Missing`, `TestHandle_Board_SchemaMismatch`, and `TestHandle_Board_Invalid` cover each branch.
- [ ] 3.5 Add a catch-all `NotFound` handler returning JSON `{"error":{"code":"NOT_FOUND","message":"..."}}` with status 404. Done when `TestHandle_Unknown_Route` asserts the response.

## 4. Browser opener

- [ ] 4.1 In `browser.go`, implement `Open(url string) error` switching on `runtime.GOOS` for `darwin` (`open`) and `linux` (`xdg-open`). Return an error for any other GOOS. Done when `go vet ./internal/server` exits 0 and a manual test (`go test -run TestOpen_DarwinSkipped` with build tags) compiles.
- [ ] 4.2 Wire `Open(url)` into `Run` after a successful bind, gated on `!opts.NoOpen`. Failures log to stderr via `fmt.Fprintln(os.Stderr, ...)` but do not abort. Done when `TestRun_NoOpen_SkipsBrowser` starts the server with `NoOpen: true` and observes that no browser command is attempted (verified via a `commandRunner` interface stubbed in tests).

## 5. Cobra wiring

- [ ] 5.1 Add `internal/commands/serve.go` declaring `NewServeCmd(jsonOut *bool) *cobra.Command` with `--port` (int, default 7777), `--no-open` (bool, default false). The `RunE` calls `server.Run` and returns the error. Done when `ezida serve --help` lists both flags.
- [ ] 5.2 Register the command in `cmd/ezida/main.go` via `rootCmd.AddCommand(commands.NewServeCmd(&jsonOut))`. Done when `ezida` (no args) lists `serve` in the command summary.

## 6. Tests

- [ ] 6.1 In `server_test.go`, add the test helpers: `startTestServer(t, boardPath string) (*httptest.Server, func())` and a fixture board file under `internal/server/testdata/valid_kanban.toml`. Done when `go test ./internal/server` discovers the file.
- [ ] 6.2 Implement every `Test*` named in sections 2, 3, 4. Done when `go test ./internal/server` exits 0.
- [ ] 6.3 Add `TestServe_BindIsLoopbackOnly` that, after `Run` chooses a port, attempts to connect to the same port via `net.Dial("tcp", "0.0.0.0:<port>")` and asserts the connection succeeds only on the loopback IP. Done when the test passes on darwin and linux.

## 7. Acceptance gate

- [ ] 7.1 Run `go test ./... && go vet ./...` from repo root. Done when exit code is 0 and the output lists every server-test name.
- [ ] 7.2 Manual smoke: run `./ezida serve --no-open` in a project with a real `kanban.toml`, `curl http://127.0.0.1:7777/api/board | jq` and confirm the payload shape. Done when the JSON matches the spec scenario and `Ctrl+C` exits within 1 s.
