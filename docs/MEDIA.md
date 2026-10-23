# Food photos and direct object storage

Menu photos belong in object storage, not PostgreSQL or the provider's filesystem.
The application owns who may attach an image to an offering; S3-compatible
storage owns the bytes. This keeps media traffic off the commerce server and
lets the same storage path serve the web application and future MCP clients.

## Authorized upload and attachment

The OpenAPI-defined media operations allocate store-owned records, issue
short-lived upload grants, verify completion, and resolve checked images.
Merchant operations use the same offering read/write permissions as menu work;
neither a UUID nor a successful storage PUT authorizes an attachment.

The Go `internal/infra/media` package uses the official AWS SDK v2 S3 presigner, not a
custom signature implementation. It binds an upload to a key, SHA-256 checksum,
content type, exact content length, and expiry. Accepted types are JPEG, PNG,
and WebP; the current limit is 10 MiB and the maximum URL lifetime is 15 minutes.
Application upload and image-read grants use five minutes. Signing itself makes
no storage request. Persistent menu revisions contain `image_id`, not a grant.

Run locally:

```text
task media:seed
task media:check
task dev:seed-media
```

S3 is available at `http://localhost:18900`; the local console is at
`http://localhost:18901`. Development-only credentials are `ocp_local_media` /
`ocp_local_media_only_123`. They are deliberately separate from OpenOM's data,
credentials, ports, and volumes. The private bucket is `ocp-demo-media`.
`task stop` retains its named volume. Do not use these credentials in production.
Use `task media:stop` to stop just storage when it is not needed.

The retained container is SILO, pinned to the supplied multi-architecture digest
in Compose. The [project repository](https://github.com/pgsty/silo) identifies it
as a MinIO fork and links its AGPL-3.0 license. It is a local development service,
not our selected production hosting product or a blanket licensing conclusion.

## Upload flow

1. A merchant requests an upload through `StartMediaUpload`.
   The server checks tenant/store ownership, quota, declared length, and type,
   allocates a unique object key, and retains an upload record.
2. `CreateMediaUploadGrant` returns a short-lived presigned PUT URL and headers.
   The browser uploads directly to storage. It echoes the signed media/checksum
   headers; the browser supplies Content-Length itself, rather than JavaScript
   attempting to set that forbidden header.
3. `CompleteMediaUpload` verifies the expected object, length, checksum, and
   decoded image type before accepting an attachment. A successful HTTP upload
   is not, by itself, permission to publish. Image validation/derivative creation
   can read from storage without routing the original upload through our API.
4. Saving an offering checks that each media ID is ready and belongs to that
   exact store, inside the revision transaction. The public image endpoint
   redirects only when the current visible publication references that ID.
   Private previews require merchant authorization. The whole bucket stays
   private; only checked derivatives receive image-read grants.

Allocation and completion use durable idempotency keys. Replay records never
contain signed URLs; grants are issued separately so retries can obtain fresh
credentials. External storage work does not hold a database transaction open.
The deterministic derivative key permits completion retry after a database
failure. UI saving is disabled during an upload so a photo cannot be silently
omitted from a saved draft.

The API receives declarations and completion requests, **not upload bytes**.
It does subsequently read the object from storage to check it. JPEG, PNG, and
WebP are decoded only after dimension checks (at most 8192 per side and 16
million pixels); at most two images are processed concurrently per process.
Images are resized to at most 960 pixels per side and re-encoded as JPEG, with
transparency flattened on white and source metadata omitted. This is not
malware scanning, an ingredient/allergen check, or an EXIF orientation feature.

Current bounded development quotas are 500 allocated assets and 200 MiB of
declared original bytes per store, serialized during allocation. Derivatives
consume additional storage. Pending allocations expire after one day, but
abandoned-object reclamation, reference-aware deletion/retention, and production
rate-limit policy still need implementation. Do not treat these limits as a
complete production abuse solution.

This keeps the UI, REST, and intent-based MCP on the same authorized media model.
Remote MCP clients may follow the direct-upload instructions; an MCP App can do
so when its host's resource/connect policy permits. Neither needs storage keys.

Presigned URLs are reusable bearer capabilities until expiry, not one-use tokens.
Do not log their query strings or include them in analytics. Use unique upload
keys, bounded quotas, verification before publication, and a future cleanup
policy. A checksum protects byte integrity, not image safety or ownership.
Keep signatures out of durable menu revisions and long-lived MCP summaries.
Previously issued image-read URLs can remain usable until their five-minute
expiry after withdrawal; publication checks do not revoke an issued capability.

OpenOM's `openom/src/storage.rs`, `S3Store`, `presign_put`, and `public_bucket`
were inspected read-only. The useful pattern is a separate client-reachable
endpoint for signing and internal endpoint for server storage operations. Do
not rewrite a signed URL's hostname. No OpenOM implementation code was copied.

## Demo assets and evidence

[The assets](../assets/demo/food/) contain synthetic photographs generated using
the built-in image-generation tool on 22 October 2026 for the fictional demo
menu. Each is associated with its own intended product, rather than one generic
image reused across different dishes. They are not actual restaurant photos or
ingredient/allergen guarantees. The demo storefront identifies them as synthetic.

The three prompt JSON files retain the exact prompt batches. `manifest.json` records intended
fixture item IDs, dimensions, JPEG sizes, source PNG hashes, delivered SHA-256
hashes, and content-addressed demo keys. No third-party stock photo was copied.
All 48 optimized JPEGs together occupy 6,263,301 bytes (about 6 MB).
Original generated PNGs remain outside tracked assets; `task media:prepare`
encodes copies placed in ignored `working/demo-media/` as JPEGs at most 768
pixels per side. Ordinary seeding needs only committed JPEGs.

`task dev:seed-media` uses shared authorized application operations to attach
prepared photos to unchanged matching demo products. It preserves existing
photos, prices, extras, availability, and custom products, checks the draft
revision, and publishes only if the old latest draft was already published.
Otherwise the image-enriched revision remains a draft. It never resets a store.
Repeating the command does not append another revision when nothing changed.
Prepared ready assets can be reused after an interrupted seed.

The food model's optional `image_id` is a coordinated draft addition to the
current schema. An older implementation with a closed schema is not thereby
compatible. New domain models must explicitly define their media references;
this field is not a universal commerce requirement.

Executed local checks include direct presigned upload/read, wrong-body checksum
rejection, changed signed type/key denial, expired URL denial, anonymous object
and bucket-list denial, and an allowed-origin CORS preflight. The test preserves
the browser-facing `localhost:18900` Host while routing TCP through Docker DNS.
Additional SILO tests read and process real objects, verify the stored JPEG and
dimensions, and reject a mismatching declared checksum. Real PostgreSQL tests
exercise allocation/completion replay, permissions, tenant/store separation,
pending/foreign attachment denial, and publication visibility, with a fake
storage processor to isolate those behaviors. Frontend tests verify that file
bytes go only to the granted destination and failed PUTs do not complete.
These checks are not an R2/S3 cloud test. See STATUS.md for the latest executed
browser and fixture-seeding results.

## Cost direction

R2 Standard is a promising initial option: its current monthly free allowance
includes 10 GB-month storage, one million Class A operations, and ten million
Class B operations, with no egress charge. Above that allowance, storage is
$0.015/GB-month; request charges still apply. These are USD list prices, not a
guarantee of a zero bill. Image transformations, other cloud services, and abuse
can add costs. See [R2 pricing](https://developers.cloudflare.com/r2/pricing/),
Standard and Free tier, checked 22 October 2026.

Keep the S3-compatible boundary, optimize images before delivery, and cache
published derivatives. This recommendation does not select the rest of our
production cloud. A production bucket, region/jurisdiction, credentials, custom
media domain, and delivery policy still need configuration and testing.

Primary implementation references: [AWS Go v2 S3 examples](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/go_s3_code_examples.html),
Presign actions; [R2 presigned URLs](https://developers.cloudflare.com/r2/api/s3/presigned-urls/),
direct uploads and signature constraints. S3 SDK v1.114.4 and credentials
v1.20.9 are pinned in the provider module. Do not infer full R2 compatibility
from this bounded SILO test.
Image processing uses the pinned `golang.org/x/image` v0.47.0
[draw](https://pkg.go.dev/golang.org/x/image/draw) and
[WebP decoder](https://pkg.go.dev/golang.org/x/image/webp); this module's BSD-3-Clause
license applies to that implementation code, not to unrelated source material.
