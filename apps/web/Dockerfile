# Tested Node 24 cohort. All frontend dependencies are pinned in package-lock.json.
FROM node:24-slim@sha256:3638d9a6fe4030bd716be989438248074489337ba3275657f93595428be4fc03 AS tools
WORKDIR /workspace/apps/web

FROM tools AS build
COPY apps/web/package*.json ./
RUN npm ci --ignore-scripts
COPY apps/web/ ./
RUN npm run check && npm run build

FROM tools AS browsers
ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright
COPY apps/web/package*.json ./
RUN npm ci --ignore-scripts
RUN npx playwright install --with-deps chromium
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
