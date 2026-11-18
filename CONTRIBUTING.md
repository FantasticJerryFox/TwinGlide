# Contributing

Keep changes small and describe the behavior a user observes.

1. Run `go fmt ./...`, `go vet ./...`, and `go test ./...`.
2. Run the race detector on macOS/Linux for coordination changes.
3. Build desktop packages on their respective operating systems.
4. Use `--no-input` and a temporary profile for network tests.
5. Never stop, replace, reconfigure or reuse an existing input-sharing installation.
6. Preserve component licenses and official dependency hash pins.
7. Update protocol, security and installation docs with behavioral changes.

Do not run an installer on a user's daily profile as a test. Native input
capture requires a separate, explicitly scheduled physical acceptance test.
Pull requests are contributed under the repository's GPL-2.0-only license.
