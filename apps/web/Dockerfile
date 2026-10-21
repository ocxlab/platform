# Tested Node 24 cohort. Local and container builds share pnpm-lock.yaml.
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
