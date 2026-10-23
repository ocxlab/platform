# Account tool contracts

Core owns merchant and customer profile intentions. Their local endpoints are
`/mcp/account/merchant` and `/mcp/account/customer`; their credentials are distinct
from each other, REST account grants, and Food commerce grants.

| Tool | Required scopes | Input | Result |
| --- | --- | --- | --- |
| `inspect_my_profile` | `account:read` | Empty object | Own name, locale, kind, ID, edit revision, and update time |
| `update_my_preferences` | `account:read`, `account:write` | Exact revision, complete editable preferences, retry key | Retained saved profile and replay flag |

Typed inputs in [accounts.go](../../../../services/platform/internal/core/adapters/mcp/accounts.go)
are schema-authoring sources for the pinned MCP SDK. Tool discovery omits writes
for a read-only grant; invocation still checks authority. Both adapters use the
same authorized application operations as REST. No payment, commerce, permission
editing, signup, or recovery tool is implied.

See [setup, transaction semantics, and limits](../../../../docs/ACCOUNT-MANAGEMENT.md).
