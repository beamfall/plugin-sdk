# Declare that a plugin's Media play alone

Ticket: CORE-3147 (`queue:core:main`). Governing ADR: ADR-0055 §2.4 and its Amendment B.
Status: proposed.

## Why

Some Media should never run on into another item when they end: a 15-hour item that rolls straight
into the next one in a list is wrong. Clients build queues from whatever list a card was shown in,
and core is domain-agnostic, so nothing tells them which Media should play alone. Resume eligibility
(`Engagement.ResumeMedia`) is not that signal, because resumable podcast episodes still advance
through a queue.

## What Changes

- `Engagement.PlaysAlone bool`: true means Media in this plugin's Libraries play alone. A host passes
  it to clients, which start such an item with a one-item queue and never auto-advance out of it or
  into it.
- No validation: the zero value keeps today's behaviour.

## Non-goals

- No per-Media or per-class override. No change to resume eligibility or to an explicit Play Queue
  a user builds.

## Failure modes

- A host that predates the field ignores it, and clients queue as they do today.

## Acceptance evidence

A unit test that the field survives bundle validation; `script/gate.sh` on the branch head.

## Rollback

Additive field: revert the commit. Bundles that never set `PlaysAlone` are unaffected.
