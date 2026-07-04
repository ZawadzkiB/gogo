---
name: gogo-contracts
user-invocable: false
description: >-
  The gogo pipeline's "type system": the registry of artifact JSON Schemas plus
  the reusable, portable validate-in / validate-out procedure every phase command
  calls. Use whenever a phase needs to validate an input it consumes or an output
  it produces (issues list, charts manifest, phase result, pipeline index) before
  hand-off — so a bad LLM hand-off is caught, not propagated.
---

# gogo-contracts — validate every hand-off

Because the phase workers are LLMs (non-deterministic), the contract + a check at
every boundary is what guarantees correct data flows to the next command. This
skill is **not a phase**; it's the shared procedure the phase skills
(`gogo-implement`, `gogo-review`, `gogo-test`, `gogo-knowledge`) call at **entry**
(validate-in) and **exit** (validate-out).

## The schema registry

All schemas live in the plugin at `${CLAUDE_PLUGIN_ROOT}/templates/contracts/`.
The human contract doc (shapes + producers/consumers) is the `README.md` there.

| Artifact (per feature folder `.gogo/work/feature-<slug>/`) | Schema | Produced by | Consumed by |
|---|---|---|---|
| `review/issues.json`, `test/issues.json` | `issues-list.schema.json` | review ③, test ④ | implement ② |
| `charts/manifest.json` | `charts-manifest.schema.json` | implement ② | review ③, test ④ |
| `<phase>/result.json` (per run) | `phase-result.schema.json` | every command | orchestrator `go` (Stage B) |
| `pipeline.json` (feature index) | `pipeline.schema.json` | every command | orchestrator `go` (Stage B) |

`plan.md` is a prose contract — its "validation" is the human acceptance gate
(`state.md` status `plan-accepted`), not a JSON schema.

## Two-tier validation (portable, no required dependency)

Every validation runs in two tiers. **Tier 1 (structural)** uses a tool only if
one is already present; **tier 2 (semantic)** the agent always runs.

### Tier 1 — structural (parse + schema), tool-detected, never installs

1. **Parse the JSON.** Use whatever exists; skip silently if none:
   ```bash
   if command -v jq >/dev/null 2>&1; then
     jq . "$FILE" >/dev/null
   elif command -v python3 >/dev/null 2>&1; then
     python3 -m json.tool "$FILE" >/dev/null
   fi
   ```
   A parse failure is a **hard contract error** — STOP (see Failure handling).
2. **Schema-check, if and only if a validator is already installed** (e.g.
   `check-jsonschema`, `ajv`). Never install one. Example (best-effort):
   ```bash
   if command -v check-jsonschema >/dev/null 2>&1; then
     check-jsonschema --schemafile \
       "${CLAUDE_PLUGIN_ROOT}/templates/contracts/<name>.schema.json" "$FILE" || true
   fi
   ```
3. If neither a parser nor a validator is present, the agent does the structural
   check itself by reading the file and the schema and confirming every required
   field, type, and enum.

### Tier 2 — semantic (the agent always runs these, tool or no tool)

Read the document and its schema and confirm:
- **Right slug** — the artifact's `slug` matches the feature folder it lives in.
- **Real paths** — every path the artifact references (inputs/outputs/artifacts,
  chart `file`s) actually exists on disk and is repo-relative (no absolute paths,
  nothing outside `.gogo/`).
- **Unique ids** — issue `id`s are unique within the list and not renumbered from
  prior rounds (cross-check against the previous snapshot when present).
- **Valid enums** — every `severity`, `priority`, `status`, `origin`, `kind`,
  `phase`, `track` value is in its allowed set (see the schema).
- **Required fields present** — including the conditional ones (a `fixed` issue
  has `fixed_in_round` + `fix_summary`).

## validate-in — the entry gate (FR2)

Before any work, for **each declared required input**:
1. Confirm the file **exists**. Missing required input → STOP with a precise
   contract error (which input, which command expected it).
2. Run **tier 1** (parse / schema-if-present) then **tier 2** (semantic) against
   the input's schema from the registry.
3. Any failure → **STOP**; never do work on bad input. Report exactly which check
   failed and where.

Optional inputs: validate the same way **if present**; absence is fine.

## validate-out — the exit gate (FR3)

After producing an artifact, before hand-off, for **each output**:
1. Run **tier 1** then **tier 2** against the output's schema.
2. On failure, **repair once** (fix the artifact you just wrote) and re-validate.
3. Still failing → mark the run **blocked**: write `result.json` with
   `status: blocked`, `validated_out: false`, and a `summary` naming the failing
   check; do **not** hand off. Surface it to the orchestrator/user.

## Failure handling — what a contract error looks like

A clear, actionable stop — name the artifact, the schema, and the exact check:

> Contract error (validate-in): `test/issues.json` — issue `TEST-002` has
> `severity: "high"`, not in {blocker, major, minor, nit}
> (`issues-list.schema.json`). Fix the producer or the file before re-running.

## Authoring a conformant artifact (for producers)

When a phase **writes** one of these, copy the field set from the schema exactly
(no extra keys — the schemas are `additionalProperties: false`), use only the
allowed enum values, keep ids stable, and write all paths repo-relative under
`.gogo/`. Then run **validate-out** on it before returning.
