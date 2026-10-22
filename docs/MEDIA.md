# Food photos and direct object storage

Menu photos belong in object storage, not PostgreSQL or the provider's filesystem.
The application owns who may attach an image to an offering; S3-compatible
storage owns the bytes. This keeps media traffic off the commerce server and
lets the same storage path serve the web application and future MCP clients.

## Implemented foundation

`task media:check` starts the optional `media` Compose profile and seeds three
synthetic food photographs through **presigned PUT URLs**. Each file is read back
through a presigned GET URL and compared byte for byte. Storage remains private.
The command repeats the batch to check repeatable seeding. It does not edit,
republish, or reset any development menu.

The Go `internal/media` package uses the official AWS SDK v2 S3 presigner, not a
custom signature implementation. It binds an upload to a key, SHA-256 checksum,
content type, exact content length, and expiry. Accepted types are JPEG, PNG,
and WebP; the current limit is 10 MiB and the maximum URL lifetime is 15 minutes.
The fixture uses five minutes. Signing itself makes no storage request.
This is a storage primitive, **not yet an authorized upload API**.

Run locally:

```text
task media:seed
task media:check
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

## Upload flow to implement next

1. A merchant requests an upload through an authorized, OpenAPI-defined operation.
   The server checks tenant/store ownership, quota, declared length, and type,
   allocates a unique object key, and retains an upload record.
2. The server returns a short-lived presigned PUT URL and required headers.
   The browser uploads directly to storage. It echoes the signed media/checksum
   headers; the browser supplies Content-Length itself, rather than JavaScript
   attempting to set that forbidden header.
3. A completion operation verifies the expected object, length, checksum, and
   decoded image type before accepting an attachment. A successful HTTP upload
   is not, by itself, permission to publish. Image validation/derivative creation
   can read from storage without routing the original upload through our API.
4. The offering references a durable owned media ID, **not an expiring URL**.
   Published resized derivatives can use a separate public/CDN delivery path;
   drafts and originals stay private. Avoid leaking draft images by granting
   public access to the entire bucket.

This keeps the UI, REST, and intent-based MCP on the same authorized media model.
Remote MCP clients may follow the direct-upload instructions; an MCP App can do
so when its host's resource/connect policy permits. Neither needs storage keys.

Presigned URLs are reusable bearer capabilities until expiry, not one-use tokens.
Do not log their query strings or include them in analytics. Use unique upload
keys, bounded quotas, cleanup of abandoned uploads, and verification before
publication. A checksum protects byte integrity, not image safety or ownership.
Keep signatures out of durable menu revisions and long-lived MCP summaries.

OpenOM's `openom/src/storage.rs`, `S3Store`, `presign_put`, and `public_bucket`
were inspected read-only. The useful pattern is a separate client-reachable
endpoint for signing and internal endpoint for server storage operations. Do
not rewrite a signed URL's hostname. No OpenOM implementation code was copied.

## Demo assets and evidence

[The assets](../assets/demo/food/) contain Döner, Margherita pizza, and falafel
photos generated using the built-in image-generation tool on 22 October 2026.
They illustrate only the corresponding fictional dishes, not every pizza or
vegetarian item. They are not actual restaurant photos or ingredient/allergen
guarantees. The future interface must identify the store as a fictional demo.

`prompts.json` retains the exact prompt set. `manifest.json` records intended
fixture item IDs, dimensions, JPEG sizes, source PNG hashes, delivered SHA-256
hashes, and content-addressed demo keys. No third-party stock photo was copied.
The three 1254-square JPEGs total 840,012 bytes. Original generated PNGs remain
outside tracked assets; `task media:prepare` can re-encode copies placed in
ignored `working/demo-media/`. Ordinary seeding needs only committed JPEGs.

Executed local checks include direct presigned upload/read, wrong-body checksum
rejection, changed signed type/key denial, expired URL denial, anonymous object
and bucket-list denial, and an allowed-origin CORS preflight. The test preserves
the browser-facing `localhost:18900` Host while routing TCP through Docker DNS.
This is not a real browser upload, a completed media API, or an R2/S3 cloud test.
No media schema field or storefront photo display has been added yet.

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
