# Development Plan & TODOs

## Stability & Robustness
- [x] **Network Timeout**: Replace `net.Dial` with `net.DialTimeout` in `components/snell/server.go` to prevent goroutine leaks when target is unresponsive.
- [x] **Relay Safety**: Change unbuffered channel to buffered `make(chan error, 2)` in `components/utils/relay.go` to prevent potential goroutine leaks.

## Performance Optimization
- [x] **Memory Allocation**: 
    - [x] Reuse buffers in `ServerHandshake` using `sync.Pool` to reduce GC pressure.
    - [x] Remove global variables in HTTP obfs server for random version numbers.
- [x] **Concurrency**: 
    - [x] Replace global `math/rand` with `crypto/rand` in TLS obfs for secure random generation.
    - [x] Replace `rand.Int()%n` with `rand.Intn()` for better concurrency.
    - [x] Remove deprecated `rand.Seed()` call.

## Code Organization
- [x] **Dependency Management**: Replace external `github.com/icpz/pool` with internal connection pool implementation.
