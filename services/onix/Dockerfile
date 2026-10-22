# Private synthetic integration only; no Beckn schema snapshot is imported.
FROM golang:1.26@sha256:26326682769ca980f8f1d3b1f52be2dd1c1d25270e3de3fe0c97d6bb65df3556 AS build
WORKDIR /src
RUN curl --fail --location --retry 2 -o /tmp/source.tgz https://codeload.github.com/beckn/beckn-onix/tar.gz/b0a134b8850a5785db6a97af8349668e664493dd \
 && echo '0dbcade7467b2a97a7af044888693a912b84687ccaf6cff81ba60d94932f2556  /tmp/source.tgz' | sha256sum -c - \
 && tar xzf /tmp/source.tgz --strip-components=1 -C /src
RUN go mod download && go mod verify
RUN mkdir -p /out/plugins /out/notices \
 && go build -trimpath -o /out/server ./cmd/adapter \
 && for plugin in registry simplekeymanager localcatalogblobstore catalogpublisher catalogcrawler; do go build -trimpath -buildmode=plugin -o /out/plugins/$plugin.so ./pkg/plugin/implementation/$plugin/cmd/plugin.go || exit 1; done \
 && go list -m -json all > /out/modules.json \
 && cp LICENSE /out/notices/ONIX-LICENSE \
 && go list -m -f '{{.Path}} {{.Dir}}' all | while read module directory; do if [ -n "$directory" ]; then name=$(printf '%s' "$module" | tr '/' '_'); mkdir -p "/out/notices/$name"; find "$directory" -maxdepth 1 -type f \( -iname 'license*' -o -iname 'notice*' -o -iname 'copying*' \) -exec cp {} "/out/notices/$name/" \;; fi; done
# The larger builder base deliberately avoids unverified libc/plugin ABI changes
# for this local test image. It is not the proposed production runtime footprint.
FROM golang:1.26@sha256:26326682769ca980f8f1d3b1f52be2dd1c1d25270e3de3fe0c97d6bb65df3556 AS local
WORKDIR /app
COPY --from=build /out/ ./
ENTRYPOINT ["/app/server"]
