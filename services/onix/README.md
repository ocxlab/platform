# ONIX network runtime integration

This component will package retained ONIX publication/crawling and the selected
catalog-core plugin seams. It is distinct from the owned provider runtime even
though the examined ONIX implementation also uses Go. This directory is not an
invented second implementation of the Beckn protocol.

The research implementation-design reference examined ONIX commit
`b0a134b8850a5785db6a97af8349668e664493dd` with catalog-core `v0.2.0`.
Those are provenance inputs for adoption, not a vendored runtime or current-head
compatibility claim. No upstream source, binaries, or schema snapshots are copied
here yet.

Integration work must retain native publisher/crawler behavior and explicitly
package their registry/key, database/geographic, and plugin needs. The owned
catalog-revision mapping and authenticated compact-index push boundary require
implementation and tests. Do not substitute a provider-local lookup and label it
native network discovery. Do not import Kafka/Discovr just because the larger demo
uses them; the compact sink seam has prior bounded experimental evidence.

Next implementation: choose verified source/image pins, record applicable code
and separately sourced schema rights, provide local configuration with synthetic
credentials, and join the first published revision to the owned discovery index.
Then add Task commands and a local composition profile. Explicit peer enrollment,
freshness, withdrawal, trust, and public participation are not established by
starting a container. This component is not started by the present smoke check.
