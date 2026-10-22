# ONIX network runtime integration

This component packages retained ONIX publication/crawling and the selected
catalog-core plugin seams. It is distinct from the owned provider runtime even
though the examined ONIX implementation also uses Go. This directory is not an
invented second implementation of the Beckn protocol.

The research implementation-design reference examined ONIX commit
`b0a134b8850a5785db6a97af8349668e664493dd` with catalog-core `v0.2.0`.
The Dockerfile downloads that exact source archive and verifies SHA-256
`0dbcade7467b2a97a7af044888693a912b84687ccaf6cff81ba60d94932f2556` before building
the native executable and plugins. The image retains `/app/modules.json` and
`/app/notices`. ONIX's root license and catalog-core's module license inspected
here are MIT; the upstream README's Apache badge conflict remains recorded in the
research material gate. No Beckn specification/schema snapshot is vendored.

Integration work must retain native publisher/crawler behavior and explicitly
package their registry/key, database/geographic, and plugin needs. The owned
catalog-revision mapping and authenticated compact-index push boundary are
implemented and tested in the bounded lab. Do not substitute a provider-local lookup and label it
native network discovery. Do not import Kafka/Discovr just because the larger demo
uses them; the compact sink seam has prior bounded experimental evidence.

Run `task test:network`; stop its containers with `task network:stop`. It starts
two native publishers, one native crawler, the owned verification/index harness,
and a separate PostgreSQL cluster, without host ports or public registration.
Ignored `working/native-onix` contains synthetic keys and must never be committed.
The ordinary development store is not reset. See
[network acceptance](../../docs/NETWORK-ACCEPTANCE.md) for executed checks,
the owned projection signature, and the limits of static internal enrollment.
