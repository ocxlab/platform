# Go 1.26.6, verified from the installed image on 2026-10-20.
# Pin a tested toolchain; a minor tag alone would move between builds.
FROM golang:1.26@sha256:26326682769ca980f8f1d3b1f52be2dd1c1d25270e3de3fe0c97d6bb65df3556 AS tools
WORKDIR /workspace/services/provider
ENV GOTOOLCHAIN=local

FROM tools AS build
COPY services/provider/go.mod ./
COPY services/provider/cmd ./cmd
COPY services/provider/internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /out/ocp-api ./cmd/api

FROM scratch AS api
COPY --from=build /out/ocp-api /ocp-api
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/ocp-api"]
