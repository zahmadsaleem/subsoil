# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.
FROM node:24.15.0-bookworm-slim AS build
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates git \
    && rm -rf /var/lib/apt/lists/* \
    && npm install --global --force pnpm@10.8.1
WORKDIR /workspace
ARG IFC_LITE_REVISION=e426ac9e5614a027367e6488e3debd29301953e6
RUN git init . \
    && git fetch --depth 1 https://github.com/LTplus-AG/ifc-lite.git "$IFC_LITE_REVISION" \
    && git checkout --detach FETCH_HEAD \
    && test "$(git rev-parse HEAD)" = "$IFC_LITE_REVISION"
COPY oci-handoff.patch /tmp/oci-handoff.patch
RUN git apply --check --whitespace=error /tmp/oci-handoff.patch \
    && git apply --whitespace=error /tmp/oci-handoff.patch
ENV VITE_REGISTRY_ENABLED=true
ENV VITE_REGISTRY_CONNECTIONS='[{"name":"Demo files","registry":"http://localhost:5001","authorization_server":"http://localhost:8080/realms/subsoil","client_id":"ifc-lite"}]'
RUN pnpm install --frozen-lockfile --ignore-scripts --store-dir /pnpm/store \
    && pnpm build:wasm:fetch \
    && pnpm build --filter=@ifc-lite/viewer \
    && cp -R apps/viewer/dist /viewer-dist \
    && rm -rf /pnpm/store node_modules packages/*/node_modules apps/*/node_modules .git
FROM nginx:1.29-alpine
COPY --from=build /viewer-dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 3000
