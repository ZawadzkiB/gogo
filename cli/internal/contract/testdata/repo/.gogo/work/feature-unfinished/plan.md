# Plan - unfinished

Status: **accepted** (user, 2026-06-28)

## Goal

A planned-but-unbuilt feature: the fixture's `unfinished` (plan column) exemplar.

## Functional requirements

- FR1 - exist as a plan-accepted work item with a WRITTEN plan.md, so the plan-readiness
  gate (0.29.0) treats it as acceptable/buildable rather than mid-authoring.

## Changes checklist

- nothing (fixture only).

## Tests

Pinned by `contract_test.go` (class = unfinished) and the `gogo status` golden.
