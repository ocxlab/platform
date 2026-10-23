# A larger local restaurant

The demo is a fictional shop, not a real restaurant account or an impersonation.
It lets us test a substantial menu before adding ordering controls and a cart.
Its own merchant keeps testing changes separate from your Döner Karlsruhe store.

## Create or find it

```text
task dev:seed
task dev:stores
```

The first command creates and publishes the 48-product menu once. It prints the
store handle and the exact credential command. Run it again safely: it preserves
store names, menu edits, publication/withdrawal, orders, and existing credentials.
It is an explicit seed, not a migration and not automatic application startup.
Its executable targets only the local Compose `ocp_dev` database.

Find **Marktplatz Küche (Demo)** under Browse a store at
<http://localhost:18181>. Browsing/guest ordering needs no merchant login.

## Manage this shop

Using the handle printed by the seed, for example:

```text
task dev:credential STORE=store-2
```

Enter the returned `merchant_id` and `token` at
<http://localhost:18181/merchant>. If already connected to another merchant,
disconnect first. The managed-store table then contains the demo shop.

For any other existing merchant, use either its store handle or its merchant UUID:

```text
task dev:credential STORE=store-1
task dev:credential MERCHANT=294e124d-5949-4f34-bae2-51caa7934df3
```

Choose only one selector. A store resolves its owning merchant; the credential
therefore covers that merchant's authorized stores, not just that location.
It is not a global provider credential. Each invocation issues a new 24-hour
development grant and leaves previous grants and store data unchanged. Unknown
selectors fail rather than silently creating another merchant.

Without a selector, `task dev:credential` retains its previous behavior: it creates
a **new** example merchant and warns about that distinction. `task dev:stores`
lists store names, handles, and owners, but no tokens. Plaintext credentials are
printed only by the explicit credential command; do not commit them.

A store ID/handle identifies a record; it is not proof of ownership. The CLI can
issue these grants because the developer already controls the local administrative
database. There is no new public login-by-store route. Production email/passkey
accounts and merchant membership can later replace this temporary bootstrap without
changing durable merchant/store identity. An email identifies a person/account,
not the restaurant's individual store or its API authorization scope.

## What to try

- Choose a large Döner, one required sauce, extra cheese, and free chili.
- Choose a large Margherita with extra cheese and free garlic: the server total
  for one is €12.00 in this fictional fixture.
- Try the plate's required side and the sold-out Baklava option.
- Open the merchant menu editor with all 48 products. This is an acceptance load
  case, not a claim that the current long editor/selector is the intended final UX.

The source lives in `services/platform/tests/fixtures/restaurant.json` and
the builder beside it. There are no real photos, payment accounts, allergens, or
restaurant promises. Intended sections are retained in fixture metadata because
the installed menu schema cannot yet represent categories. See the
[benchmark and design recommendations](RESTAURANT-UX-BENCHMARK.md) for the real-menu
probe, gaps, and the proposed progressive-disclosure approach.
