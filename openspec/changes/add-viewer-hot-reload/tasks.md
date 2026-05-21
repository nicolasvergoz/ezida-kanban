## 1. Dependency

- [ ] 1.1 Run `go get github.com/fsnotify/fsnotify@latest` and verify `go.mod` lists exactly one new direct dependency (fsnotify) and `go.sum` is updated. Done when `go mod tidy` exits 0 with no further changes.

## 2. Watcher

- [ ] 2.1 Create `internal/server/watcher.go` declaring `type Watcher struct { ... }`, `NewWatcher(path string) (*Watcher, error)`, `Run(ctx context.Context)`, and `Events() <-chan struct{}`. Done when `go vet ./internal/server` exits 0.
- [ ] 2.2 Implement the 200 ms debounce per design. Done when `TestWatcher_DebouncesBurst` (writes 3× within 100 ms, asserts ≤1 event in 500 ms) passes.
- [ ] 2.3 Implement re-arm on Rename (call `fsw.Add(path)` after each Rename/Create event, swallowing `ErrExist`-style errors). Done when `TestWatcher_SurvivesRename` (two atomic rewrites 1 s apart, asserts 2 events) passes.
- [ ] 2.4 Add `TestWatcher_ShutsDownOnCancel` asserting `Run` returns within 100 ms of `ctx` cancellation. Done when the test passes.

## 3. Broker

- [ ] 3.1 Create `internal/server/sse.go` declaring `type Broker struct { ... }`, `NewBroker() *Broker`, `Subscribe() (chan Event, func())`, `Broadcast()`. Use a `sync.Mutex` per design. Done when `go vet` passes.
- [ ] 3.2 Implement `Subscribe`'s `unsubscribe` closure to remove the channel and close it. Done when `TestBroker_UnsubscribeClosesChannel` passes.
- [ ] 3.3 Implement `Broadcast` with non-blocking sends (default drop). Done when `TestBroker_SlowClientDoesNotBlockOthers` passes.

## 4. SSE handler

- [ ] 4.1 In `internal/server/handlers.go`, add `handleEvents` per design (write `retry: 2000` on connect, set headers, register heartbeat ticker, loop on broker channel + r.Context().Done()). Done when `TestHandle_Events_SendsRetryHeader` asserts the first chunk contains `retry: 2000`.
- [ ] 4.2 Register `GET /api/events` on the server's `ServeMux`. Done when `curl -N http://127.0.0.1:<port>/api/events` keeps the connection open and prints `retry: 2000`.
- [ ] 4.3 Add `TestHandle_Events_BroadcastsBoardChanged` that subscribes a client, triggers a broker broadcast, and asserts the client received `event: board-changed`. Done when the test passes.
- [ ] 4.4 Add `TestHandle_Events_HeartbeatTickerWorks` (with the ticker interval shrunk to 100 ms via a test hook) asserting the client receives `: ping` within 200 ms of idle. Done when the test passes.

## 5. Server wiring

- [ ] 5.1 Extend `internal/server/server.go` `Run` to construct the `Watcher` and `Broker`, start them on goroutines bound to the signal context, and forward watcher events to broker broadcasts. Done when `TestRun_HotReload_Smoke` boots a server against a fixture, rewrites the board, opens an EventSource client via `net/http` raw streaming, and observes a `board-changed` event.
- [ ] 5.2 If `NewWatcher` returns an error (e.g. board file missing), `Run` MUST return the error to the caller without binding the HTTP listener. Done when `TestRun_FailsIfBoardMissing` asserts the error path.
- [ ] 5.3 On shutdown, ensure the broker has no leaked goroutines (subscribed clients all return). Done when `TestRun_ShutdownReleasesClients` passes (subscribe, send signal, assert client loop exits within 5 s).

## 6. UI EventSource

- [ ] 6.1 Extend `internal/server/web/app.js` `board()` with `connected: false`, `eventSource: null`, `connectEvents()`, `handleExternalChange()`. Done when `app.js` defines them in the returned object literal.
- [ ] 6.2 Call `connectEvents()` from `load()` after the first successful load (or from `init` after the first `load()` completes). Done when manual smoke shows the dot turning green within 500 ms of page load.
- [ ] 6.3 In `handleExternalChange`, call `closeCard()` (no-op if not editing) then `load()`. Done when manual smoke confirms an open modal closes when a CLI write fires.

## 7. UI status dot

- [ ] 7.1 In `internal/server/web/index.html`, add `<span class="status-dot" :class="connected ? 'on' : 'off'"></span>` inside the topbar after the project name span. Done when the rendered DOM contains the element.
- [ ] 7.2 In `internal/server/web/style.css`, add the `.status-dot`, `.status-dot.on`, `.status-dot.off` rules per design. Done when the dot is visible and color changes between the two states.

## 8. Manual smoke

- [ ] 8.1 Terminal A: `./ezida serve`. Terminal B: `./ezida add "Hot reload test" --column=todo`. Confirm the card appears in the browser within 2 s. Done when verified.
- [ ] 8.2 Edit `kanban.toml` manually in a text editor, change a card's title, save. Confirm the browser reflects the change within 2 s. Done when verified.
- [ ] 8.3 With the browser open, kill the server with Ctrl+C. Confirm the topbar dot turns gray within 5 s. Restart the server on the same port. Confirm the dot turns green again without a manual reload. Done when verified.
- [ ] 8.4 Open the edit modal on a card, then in another terminal `ezida move <id> done`. Confirm the modal closes and the card appears in the `done` column. Done when verified.

## 9. Acceptance gate

- [ ] 9.1 Run `go test ./... && go vet ./...`. Done when exit code is 0 and every new test name appears in the output.
- [ ] 9.2 Run `go mod tidy` once more and confirm no further changes are required. Done when the command produces no diff.
