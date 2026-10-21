# Protocol review of admin browsing

22 October 2026, Europe/Berlin (UTC+02:00). Automated static review of the current
admin-directory and extras UX batch by the existing protocol reviewer. This is
not independent interoperability evidence or an external certification.

The reviewer checked provider-only list routing, independent SQL grant checks,
resource/query/status-bound cursors, summary minimization, forced tenant RLS,
hidden row identifiers, and schema-valid extras defaults. One correction was
required: the two new GET contracts needed explicit expected 400, 401, 403, and
503 Problem responses rather than only a default error response.

Both path files now declare those statuses and retain the fallback default.
Go and TypeScript were regenerated. The reviewer rechecked the change and
reported no remaining protocol blocker pending runtime verification. The new
integration cases also check filter-bound order continuations and readable
references beyond six digits. Runtime results belong in [status](../STATUS.md),
not in a claim that static review establishes application behavior.
