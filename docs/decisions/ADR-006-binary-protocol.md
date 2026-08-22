# ADR-006: Binary Protocol with JSON Debug Mode

**Status:** Accepted  
**Date:** 2026-08-22

## Context
High-frequency chunk updates at 20 Hz with 1M cells create significant bandwidth. JSON encoding is 5-10× larger than binary.

## Decision
BinaryEncoderV1 for production. DebugJSONEncoder for development. UpdateEncoder interface allows switching at startup.

## Alternatives Considered
- JSON only (too much bandwidth)
- Protobuf (adds dependency, less control)
- MessagePack (middle ground but not as compact as raw binary)

## Rationale
Binary encoding with version tags provides minimal overhead. The interface makes protocol evolution non-breaking. Debug JSON aids browser console inspection during development.

## Consequences
- Client must decode binary messages
- Protocol version byte enables forward compatibility
- Changing encoding does not require simulation or game code changes

## References
- WorldWeaver_Full_Project_Documentation.md § 22.3
- Architecture Addendum § 27
