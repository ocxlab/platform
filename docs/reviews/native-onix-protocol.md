# Native ONIX discovery seam protocol review

Reviewed 22 October 2026, Europe/Berlin. This is a static protocol/trust-boundary
review of the uncommitted native ONIX integration. The parent reports that
`TestNativeONIXFoodJourney` passed in 10.87 seconds and its race run in 11.918
seconds; this reviewer did not run Docker, native ONIX, PostgreSQL, or race
checks.

## Reviewed boundary

The slice is a private synthetic discovery experiment. It does not implement
Beckn checkout, join a public fabric, or make a production trust claim. Its
intended path is:

1. A publication transaction commits an immutable, public-only `CatalogIntent`
   with an outbox effect keyed by store and publication generation.
2. The owned publisher derives an immutable catalog ID from the store UUID and
   generation, attaches a typed OCP projection, and signs that projection.
3. Pinned native ONIX publisher/crawler components distribute the catalog.
4. The isolated owned ingress verifies the OCP projection against static
   enrollment, validates its public model projection, and accepts only a
   numerically newer generation into the compact index.
5. Search resolves the stored OCP API binding; the Food service, rather than
   ONIX catalog data, remains the authoritative quote/pricing endpoint.

## Findings and disposition

| Severity | Finding | Disposition |
| --- | --- | --- |
| High | The retained native crawler's callback context identifies the crawler's configured participant, not the original native artifact signer. Treating `payload.provider.id` as a verified original signer would have created a false trust boundary. | Resolved by the owned Ed25519 projection signature. The index verifies the signed typed projection against a static enrollment binding rather than inferring authority from crawler context. |
| Medium | Out-of-order publication retries could let an older catalog revive a withdrawn or newer store record. | Resolved. The index uses numerical generation ordering under a per-store advisory lock; equal generations are idempotent only for the same digest/body, while changed content conflicts. Withdrawal is a higher-generation hidden tombstone and later republish requires a further generation. |
| Medium | An action label alone does not determine public visibility: a merchant can select a revision while a provider suspension remains active. | Intended behavior, not a defect. `publish + visible:false + document:null` records the merchant's retained selection while suspension still hides it. Add/retain an intent-matrix regression for that case, restore, withdraw, and suspend so future cleanup does not incorrectly equate `publish` with visibility. |
| Medium | A native signature and a separately signed OCP projection may be confused as one trust assertion. | Bounded correctly in this slice: native artifact handling is a crawler transport concern; owned index admission relies on the separately verified OCP projection and static enrollment. Do not describe native signature verification alone as authorization to route a customer. |
| Low | The projection byte representation is an owned integer-JSON canonicalization profile, not RFC 8785 JCS. | Acceptable for this fixed local Go producer/verifier pair. Version-prefix it as implemented, retain byte-vector tests, and do not offer independent signer interoperability until a published canonicalization profile and test vectors exist. |

## Static assessment

The code keeps the important authority boundaries visible:

- `CatalogIntent` is created in the same publication transaction and carries
  only visibility, generation, retained model/revision, display name, and model
  projection. Drafts, guest authority, orders, payment data, and private
  merchant records are not in this seam.
- `Verify` requires one complete catalog resource; exact enrolled provider,
  merchant, store, API base, and public key; an immutable catalog/resource ID;
  an active catalog; and a valid OCP projection signature.
- `Index.Accept` validates model ownership and checks that the supplied document
  is exactly the approved model projection before persisting it. A hidden
  tombstone must contain `null`, not a retained public document.
- The index's generation comparison is in PostgreSQL, not a worker-memory
  cache. A late old artifact cannot overwrite the current index state.
- Search only returns visible entries observed in the last 24 hours. Its result
  is discovery metadata, not a purchase authorization or a price commitment.

One important local-lab limitation remains: the synthetic setup uses static
enrollment and keeps generated private keys/configuration in ignored local
runtime storage. It has no key rotation, enrollment revocation, remote-peer
authorization policy, availability SLA, or public ingress hardening. The
crawler push endpoint is Docker-network-only by deployment convention; the
projection signature prevents forged catalog admission but is not a complete
DoS/access-control layer.

## Material and source boundary

The retained native source pin is `beckn/beckn-onix`
`b0a134b8850a5785db6a97af8349668e664493dd`. The Dockerfile fetches that exact
tarball and verifies its recorded digest, retains direct-module license/notice
files in the local image, and does not vendor Beckn core schema/API material.
The ONIX root license was observed as MIT while a README badge previously
indicated Apache-2.0; preserve both notices and record the reconciliation before
any image distribution claim. The integration must not copy or derive the
separately reviewed Beckn core schema pin, whose material-rights status remains
unresolved for commercial product use.

The product uses `catalog-core v0.2.0` only where retained analysis recorded
that module/version, not an inspected newer head. A fresh image/source/archive
and module graph digest must accompany any later rebuild; the old experimental
images and their disabled-validation/authentication configuration are not a
production dependency baseline.

## Verdict

**Static acceptance for the bounded synthetic native discovery seam, pending the
parent-run full checks and the publication/suspension intent-matrix regression.**
The reported native test is meaningful evidence that this exact local path can
publish, crawl, search, update, withdraw, and reach an authoritative Food quote.
It is not evidence of public ONIX/Beckn federation, merchant identity trust,
checkout interoperability, or a commercially cleared Beckn schema integration.
