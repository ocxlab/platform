# Engineering review of admin browsing

22 October 2026, Europe/Berlin (UTC+02:00). Automated static review of the
uncommitted admin-directory and extras UX batch by the existing engineering
reviewer. This is not an independent human review or a browser/usability study.

The review covered the migration's read-only cross-tenant projections, provider
audience routing, scoped list operations, cursor handling, admin table navigation,
and the menu editor's existing extras model. It identified one required test gap:
both new SECURITY DEFINER functions needed owner, fixed search-path, runtime
execution, and PUBLIC-execution regression assertions, matching the existing
public-directory hardening tests.

`TestAdminStoreDirectory` now checks both exact function signatures against
`pg_proc`, `pg_roles`, and ACL expansion. The reviewer re-read those assertions
and confirmed that the finding was resolved. No other material static blocker
was reported. Runtime results are recorded separately in [status](../STATUS.md).
