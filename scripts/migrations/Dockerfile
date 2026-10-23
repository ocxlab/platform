FROM ghcr.io/sbdchd/squawk:2.70.0@sha256:95707f5660dfadbc61cf9d9565384b54d157dcb4e8d3c30db78dd7056be293e9 AS squawk
FROM golang:1.26@sha256:26326682769ca980f8f1d3b1f52be2dd1c1d25270e3de3fe0c97d6bb65df3556 AS goose
WORKDIR /src
COPY services/platform/go.mod services/platform/go.sum ./
RUN go mod download
COPY services/platform/cmd/test/migration-replay/main.go ./main.go
RUN CGO_ENABLED=0 go build -o /migration-replay ./main.go
FROM mcr.microsoft.com/devcontainers/python:3.13@sha256:15f58ad71b05185d37052bf965024e87a206c14812f037e52093d5ee3d83cd80
USER root
RUN python -m venv /opt/migrations && /opt/migrations/bin/pip install --no-cache-dir psycopg2-binary==2.9.11
COPY --from=squawk /usr/local/bin/squawk /usr/local/bin/squawk
COPY --from=goose /migration-replay /usr/local/bin/migration-replay
RUN curl -fsSL https://github.com/pgplex/pgschema/releases/download/v1.13.3/pgschema-1.13.3-linux-amd64 -o /usr/local/bin/pgschema \
    && echo '83d5c6c08e19cdf09bbc00c44aa60ae7fd6047760e4107f17182a60915f2ae14  /usr/local/bin/pgschema' | sha256sum -c - \
    && chmod +x /usr/local/bin/pgschema
ENV PATH="/opt/migrations/bin:${PATH}" PYTHONDONTWRITEBYTECODE=1
WORKDIR /workspace
ENTRYPOINT ["python", "/workspace/scripts/migrations/workflow.py"]
