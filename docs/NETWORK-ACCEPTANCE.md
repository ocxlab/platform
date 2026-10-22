# Native discovery acceptance

22 October 2026. This checkpoint answers whether retained ONIX software can carry
our Food publications between independently configured providers and route a
customer back to the correct commerce API. It does not test public participation
or a Beckn checkout implementation.

## Local topology

`task test:network` builds the pinned ONIX image, initializes synthetic enrollment,
and starts two native publishers, one native crawler, the owned test harness, and
a PostgreSQL cluster. Each commerce provider has its own database and ordinary
authorized HTTP implementation. The owned discovery index has a separate database;
the native crawler also owns its catalog database. These are test boundaries, not
a requirement for four production database servers.

Publication commits an immutable public projection and durable delivery intent
with the store decision. A bounded worker sends it to the native publisher. The
crawler obtains the native artifacts and pushes them through the owned verification
boundary. Search returns public metadata and the enrolled merchant endpoint, never
private orders or an authoritative purchase price.

## Executed evidence

`TestNativeONIXFoodJourney` passed against the actual containers, with Go's race
detector (latest rerun: 10.44-second journey, 11.514-second package run). It exercised:

- publishing and discovering Food catalogs from both providers;
- routing discovered identities to authoritative quotes from their respective APIs;
- updating a price through publication, crawl, and discovery;
- rejecting stale generation requests and a quote sent to the wrong provider; and
- propagating a withdrawal tombstone so the store disappears and no longer quotes.

Separate PostgreSQL/race tests exercised signed projection verification, changed
same-generation rejection, monotonic update handling, visibility tombstones, and
stale-entry hiding. The native rerun includes freshness renewal on an authenticated
duplicate, narrower container key mounts, and all 18 current migrations. A separate
catalog-worker process also consumed provider A's outbox during this run; leased
work remained coordinated with the harness worker. The full regression result is
recorded in `STATUS.md`, not inferred from the native run.

## Authority and freshness

The native crawler's push context identifies its own deployment, not the original
merchant host. Consequently, the owned projection carries an Ed25519 signature
bound to the statically enrolled provider, merchant, store, endpoint, key, and
publication generation. Verification also checks the exact installed offering
model and its safe public projection. It does not trust rewritten crawler context
as original signing authority.

Signing uses a versioned owned JSON serialization for this bounded integer-valued
projection, not a general RFC 8785 implementation. Native catalog IDs include the
generation, so an old delivery cannot overwrite the newer artifact. Withdrawal
uses a signed hidden projection carried by an active native catalog; this is not
a claim that native catalog retirement was tested.

Authenticated duplicate receipts refresh `observed_at`. Search hides entries after
24 hours without a verified receipt. The native crawler need not push an unchanged
catalog again, so active authenticated revalidation remains release work; the
stale-hide policy is deliberately conservative, not continuous freshness proof.

## Operational limits

Native publisher/crawler containers receive only their own configuration/key
material. The trusted local harness has broader fixture access and is not a
production public-ingress deployment. Enrollment, registry responses, and private
HTTP origins are explicit local fixtures; public key rotation, hostile peer joining,
merchant migration, broad crawler interoperability, and production capacity are
not established.

One quiet `docker stats` sample showed roughly 14 MiB for the owned harness,
17–18 MiB per native publisher, 18 MiB for the crawler, and 90 MiB for PostgreSQL.
This is not a load benchmark or cost estimate. The native development image keeps
the build toolchain for plugin ABI consistency and is approximately 1.5 GB locally;
production image reduction remains separate work.
