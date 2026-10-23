# Tested Node 24 cohort. Local and container builds share pnpm-lock.yaml.
FROM golang:1.26@sha256:26326682769ca980f8f1d3b1f52be2dd1c1d25270e3de3fe0c97d6bb65df3556 AS contracts
WORKDIR /workspace/services/platform
ENV GOTOOLCHAIN=local
COPY services/platform/go.mod services/platform/go.sum ./
RUN go mod download
COPY services/platform/cmd/contracts ./cmd/contracts
COPY contracts/openapi/v1 /workspace/contracts/openapi/v1
RUN go run ./cmd/contracts

FROM node:24-slim@sha256:3638d9a6fe4030bd716be989438248074489337ba3275657f93595428be4fc03 AS tools
RUN npm install --global pnpm@11.20.0
WORKDIR /workspace/apps/web

FROM tools AS mutation-tools
# Stryker uses ps to reap timed-out test-worker process trees. Keep this
# tooling dependency out of the normal application/runtime stages.
RUN apt-get update && apt-get install -y --no-install-recommends procps && rm -rf /var/lib/apt/lists/*

FROM tools AS build
COPY apps/web/package.json apps/web/pnpm-lock.yaml apps/web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile --ignore-scripts
COPY apps/web/ ./
COPY --from=contracts /workspace/contracts/build/provider.openapi.json /workspace/contracts/build/provider.openapi.json
COPY contracts/errors /workspace/contracts/errors
COPY scripts/generate-errors.mjs /workspace/scripts/generate-errors.mjs
RUN node /workspace/scripts/generate-errors.mjs && pnpm run contracts:generate
RUN pnpm run check && pnpm run build

FROM tools AS browsers
ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright
COPY apps/web/package.json apps/web/pnpm-lock.yaml apps/web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile --ignore-scripts
RUN pnpm exec playwright install --with-deps chromium
COPY apps/web/ ./
CMD ["node", "scripts/run-browser-tests.mjs"]

FROM tools AS web
ENV NODE_ENV=production HOST=0.0.0.0 PORT=3000
COPY --from=build --chown=node:node /workspace/apps/web/build ./build
COPY --from=build --chown=node:node /workspace/apps/web/node_modules ./node_modules
COPY --from=build /workspace/apps/web/package.json ./
USER node
EXPOSE 3000
CMD ["node", "build/index.js"]
