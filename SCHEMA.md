# Vector Schema (Linea Conformance Suite)

This document is **normative** for the suite. Every JSON file under `vectors/`
follows this schema. Implementations consuming the suite MUST treat unknown
fields as test-vector errors, not as silently ignorable.

## Top-level shape

```jsonc
{
  "id":          "ranking/01-prefer-higher-certainty",  // unique, stable, kebab-case
  "spec":        "CCGGS-1.1.0",                         // spec doc + version
  "section":     "9.3",                                 // spec section reference
  "description": "Higher path certainty ranks first",   // one-line human summary
  "setup":       { /* see below */ },                   // initial graph
  "operation":   { /* see below */ },                   // what to execute
  "expected":    { /* see below */ }                    // what must come back
}
```

`id` is the relative path of the file under `vectors/` without the `.json`
extension. Runners SHOULD verify this invariant.

## `setup`

Builds the initial graph. All sub-sections are optional.

```jsonc
{
  "persons": [
    { "id": "A" },
    { "id": "B", "names": [{ "text": "Alice", "type": "given", "preferred": true }] },
    { "id": "U", "unknown": true }                       // unknown-ancestor placeholder
  ],
  "relationships": [
    {
      "id":         "r1",
      "type":       "ParentChild",                      // ParentChild | Marriage
      "from":       "A",
      "to":         "B",
      "certainty":  "Certain",                          // Certain | Probable | Uncertain
      "continuity": { "state": "Continuous" }           // see continuity below
    }
  ],
  "sources":   [],
  "proposals": []
}
```

### Continuity sub-object

```jsonc
{ "state": "Continuous" }                                // simple
{ "state": "Gapped", "gapKnown": true,  "gapSize": 3 }   // known-size gap
{ "state": "Gapped", "gapKnown": false }                 // unknown-size gap
```

Marriage edges MUST always use `Continuous`.

### Unknown-ancestor placeholder

A person with `"unknown": true` MUST NOT carry `names`, `gender`, `birth`,
`death`, or `notes`. The runner enforces this; vectors violating it are
test-suite bugs, not implementation bugs.

### Proposals

Proposals follow the same shape as the model package would emit:

```jsonc
{
  "id":         "p1",
  "state":      "Draft",                                // see CCGGS §8.3
  "action":     "Create",                               // Create | Update | Retract | Merge | SameAsLink
  "entityKind": "Person",                               // Person | Relationship | Source
  "targetId":   "A",                                    // optional
  "secondaryId": "B",                                   // optional, used by Merge / SameAsLink
  "payload":    { /* action-specific JSON */ },
  "reason":     "...",
  "author":     "..."
}
```

## `operation`

Tells the runner what to invoke.

```jsonc
{ "kind": "CertaintyAlgebra", "input": ["Certain", "Probable", "Uncertain"] }

{ "kind": "FindPaths", "from": "A", "to": "C",
  "options": { "includeAffinal": false, "maxDepth": 16, "maxPaths": 0 } }

{ "kind": "NKCA",      "a": "A", "b": "B" }

{ "kind": "ProposalTransition", "proposalId": "p1",
  "to": "Submitted", "actor": "alice", "timestamp": 10, "reason": "" }
```

The exhaustive set of supported operations is enumerated under
[`vectors/`](./vectors) — runners MAY support a subset, but MUST report
unsupported `kind` values as a test failure (not silently skip).

## `expected`

```jsonc
// Success outcome
{
  "outcome": "ok",
  "result":  { /* shape depends on operation.kind */ }
}

// Error outcome
{
  "outcome": "error",
  "code":    "NO_KNOWN_CONNECTION"
}
```

### Result shapes by operation kind

#### `CertaintyAlgebra`
```jsonc
{ "result": "Probable" }            // the weakest-link of the inputs
```

#### `FindPaths`
The runner SHOULD produce the same number of paths as `result.paths`, in
order. Each entry is compared by:
```jsonc
{
  "from":           "A",
  "to":             "C",
  "length":         2,
  "certainty":      "Certain",
  "totalGap":       0,
  "gapEdges":       0,
  "classification": "lineage"        // lineage | affinal
}
```
Path identity (which exact edges) is checked only when `result.exact == true`.

#### `NKCA`
```jsonc
{
  "ancestorId":        "GP",
  "ancestorIsUnknown": false,
  "totalGenerations":  4,
  "combinedCertainty": "Certain"
}
```

#### `ProposalTransition`
```jsonc
{ "newState": "Submitted" }
```

## Spec-version pinning

The suite version is declared in [`version.json`](./version.json):
```json
{ "spec": "v1.1.0", "suite": "0.1.0" }
```

Conformance badges and reports SHOULD cite both.
