# Customer Food tool contract

The local Streamable HTTP endpoint is `/mcp/customer/food`. Its resource-bound
guest credential authorizes checkout at one store; it is not an account login,
OAuth token, browser guest token, or payment credential.

| Tool | Customer intention | Result boundary |
| --- | --- | --- |
| `find_food_stores` | Find a named public store | Up to 20 local summaries, no private data |
| `inspect_store_menu` | Narrow the published menu | Up to 20 dishes, pinned generation |
| `inspect_food_dish` | Choose sizes and extras | One dish, optionally up to three expanded groups |
| `inspect_food_meals` | Inspect meal upgrades | Paginated offer summaries or one offer's choices |
| `price_food_basket` | Price complete intended selections | Owned, expiring quote handle and exact terms |
| `inspect_food_basket` | Review a priced selection | Owned quote, lines, notes, and total |
| `revise_food_basket` | Replace, remove, or add selected lines | A new immutable quote handle |
| `commit_food_order` | Place the agreed pickup order | Retained order summary; explicit confirmation and total required |
| `track_my_food_order` | Check progress | Summary; lines and history only when requested |
| `cancel_my_food_order` | Cancel an eligible owned order | Revision-guarded retained outcome |

All tool names and schemas derive from the typed inputs in
[customer.go](../../../../services/platform/internal/food/adapters/mcp/customer.go).
The pinned Go MCP SDK validates those schemas. Revisions, tenant scope, checkout
rules, and retries use the shared Food application and persistence operations,
not a parallel MCP implementation.

Publication generations bind paginated reads to a menu version. A `basket_id`
is an immutable quote, not a reserved cart or a paid order. Retrying a committed
operation uses the same 16–128 character request key; do not invent a new key to
resolve an uncertain outcome. Confirmation is an input requirement, not proof
that a human consented. No tool here charges money.

See [setup, examples, and limits](../../../../docs/CUSTOMER-MCP.md).
