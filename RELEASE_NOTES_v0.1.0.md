# Release v0.1.0

Initial public release of `httpx`.

## Highlights
- Minimal `Request` helper with JSON/XML/plain body support.
- Optional retry logic for idempotent methods.
- Custom `http.Client` injection.
- CI pipeline and test coverage for retry/client behaviors.

## Notes
- Default timeout is 15s; you can override per call.
- Retry applies to 429/5xx and network timeouts.
