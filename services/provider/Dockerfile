# Go 1.26.6, verified from the installed image on 2026-10-20.
# Pin a tested toolchain; a minor tag alone would move between builds.
FROM sqlc/sqlc:1.31.1@sha256:70f53171d27b2424e9358869975455a6e955a5aa8e58a998a270a6e34e525537 AS sqlc

FROM golang:1.26@sha256:26326682769ca980f8f1d3b1f52be2dd1c1d25270e3de3fe0c97d6bb65df3556 AS tools
COPY --from=sqlc /workspace/sqlc /usr/local/bin/sqlc
WORKDIR /workspace/services/provider
ENV GOTOOLCHAIN=local

FROM node:24-slim@sha256:3638d9a6fe4030bd716be989438248074489337ba3275657f93595428be4fc03 AS errors
WORKDIR /workspace
COPY contracts/errors ./contracts/errors
COPY scripts/generate-errors.mjs ./scripts/generate-errors.mjs
RUN node scripts/generate-errors.mjs

FROM tools AS build
COPY services/provider/go.mod services/provider/go.sum ./
RUN go mod download
COPY services/provider/cmd ./cmd
COPY services/provider/internal ./internal
COPY services/provider/sqlc.yaml ./sqlc.yaml
RUN sqlc generate
COPY contracts/openapi/v1 /workspace/contracts/openapi/v1
COPY contracts/openapi/internal /workspace/contracts/openapi/internal
COPY contracts/tooling/go.yaml /workspace/contracts/tooling/go.yaml
COPY contracts/tooling/internal-go.yaml /workspace/contracts/tooling/internal-go.yaml
COPY --from=errors /workspace/services/provider/internal/problems/catalog.generated.json ./internal/problems/catalog.generated.json
RUN go run ./cmd/contracts && go tool oapi-codegen --config ../../contracts/tooling/go.yaml ../../contracts/build/provider.openapi.json
RUN go run ./cmd/contracts -source ../../contracts/openapi/internal/openapi.yaml -output ../../contracts/build/internal.openapi.json && go tool oapi-codegen --config ../../contracts/tooling/internal-go.yaml ../../contracts/build/internal.openapi.json
RUN CGO_ENABLED=0 go build -trimpath -o /out/ocp-api ./cmd/api

FROM scratch AS api
COPY --from=build /out/ocp-api /ocp-api
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/ocp-api"]
