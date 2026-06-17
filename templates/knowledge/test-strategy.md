# Test strategy

**Purpose:** how to test a change — journeys, UI / design checks, e2e levels,
deployment checks, and the done-bar. (The tools themselves are in
`testing-tools.md`.)

<!-- gogo:meta
Mode: owned
Source: [ ]
Confidence: low
Generated-by: /gogo:build (scaffold)
-->
> How to test, level by level. Verify the bars in `non-functional-requirements.md`.

## Levels
- **Unit / integration** — <where; what's pinned (pure logic, deterministic shapes)>.
- **e2e** — <how to seed / isolate; what each level asserts>.

## How to test a change (per level it touches)
- **UI** → drive real clicks / flows with the browser tooling; assert the journey
  AND that it looks right (matches the design); explore edges, not just the happy path.
- **API** → hit endpoints (status, shape, errors).
- **CLI** → run the commands; assert stdout / exit code.

## Key user journeys
<the journeys to walk; what "looks right" means for this project>

## Deployment checks
<build clean, app boots, smoke the critical path; "n/a" for a library>

## Done bar
Build clean AND all unit AND all e2e green, PLUS hands-on exploration of the
actual change (not just green tests).
