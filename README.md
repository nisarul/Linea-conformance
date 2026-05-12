# Linea Conformance Suite

Language-agnostic test vectors that any [Linea](https://github.com/nisarul/Linea-specs)
implementation can use to claim conformance to the Linea Specifications.

The suite is **the contract**. If a vector here passes against your implementation,
your implementation behaves correctly with respect to the spec section the vector
covers. If a vector fails, your implementation is non-conformant.

> Linea — lineage, without assumptions.

## Layout

```
Linea-conformance/
├── version.json          # spec version + suite version
├── SCHEMA.md             # vector schema (read this first)
├── vectors/              # the test vectors, grouped by spec area
│   ├── 01-certainty-algebra/
│   ├── 02-path-ranking/
│   ├── 03-find-paths/
│   ├── 04-nkca/
│   ├── 05-no-connection/
│   ├── 06-proposal-lifecycle/
│   ├── 07-cycle-detection/
│   └── 08-unknown-ancestor/
└── runner-go/            # reference Go runner that executes the suite against Linea-core
```

## Versions

- **Spec version:** the Linea Spec tag the vectors target (currently `v1.1.0`).
- **Suite version:** independent; bumps when vectors are added, fixed, or
  reorganised. Conformance reports SHOULD include both.

## How to consume

### Go

The reference implementation lives at
[`runner-go/`](./runner-go). It depends on `github.com/nisarul/Linea-core`
and runs every vector against an injected `store.Store` factory.

```sh
cd runner-go
go test ./...
```

### Other languages

Vectors are plain JSON. To consume them in any language:

1. Read [`SCHEMA.md`](./SCHEMA.md) for the field-by-field semantics.
2. Walk `vectors/**/*.json`.
3. For each vector:
   - Build a fresh empty graph in your implementation.
   - Apply `setup` (persons, relationships, sources, proposals).
   - Run `operation`.
   - Compare the result to `expected` using the comparison rules in `SCHEMA.md`.

## Contract

- **Stable IDs.** Vectors use string aliases (e.g. `"A"`, `"GP"`). The runner
  maps each alias to a stable internal ID for the run. Implementations MUST
  preserve referential integrity across `setup` and `expected`.
- **No fabrication.** Tests assume implementations follow CCGGS §5.3:
  unknown-ancestor placeholders carry no fabricated attributes.
- **Semantic outcomes.** Errors are checked by `code` (e.g. `NO_KNOWN_CONNECTION`),
  never by error message text.

## License

AGPL-3.0-or-later. See [LICENSE](./LICENSE).

The Linea specifications themselves are licensed under CC BY 4.0.
