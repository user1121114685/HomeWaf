# HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

# HomeWaf source build with pinned base images; update digests deliberately.
FROM node:26.13.0-alpine@sha256:8996bb5821d62a8fd9bb27c1cb07462f3fdce6e47d640c6a881ead0b064ce216 AS web
WORKDIR /src
COPY scripts/prepare-ip-databases.mjs ./scripts/
COPY cmd/homewaf/exedata/ip2region_v6.xdb cmd/homewaf/exedata/ip2region_v6.source.json ./cmd/homewaf/exedata/
RUN node scripts/prepare-ip-databases.mjs --check
COPY web/package*.json ./web/
COPY web/scripts ./web/scripts
COPY web/vendor ./web/vendor
RUN npm ci --prefix web
COPY web ./web
RUN npm run typecheck --prefix web && npm run build --prefix web

FROM golang:1.27.3-alpine@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18 AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./public/dist
RUN mkdir -p public/dist/legal && cp LICENSE NOTICE THIRD_PARTY_NOTICES.md public/dist/legal/ && cp -R LICENSES public/dist/legal/ && printf 'version=2.0.0\nsource=local-package-lock\n' > public/web_frontend.txt
ARG HOMEWAF_VERSION=v2.0.0-dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X HomeWaf/global.GWAF_RELEASE=true -X HomeWaf/global.GWAF_RELEASE_VERSION=${HOMEWAF_VERSION}" -o /out/homewaf ./cmd/homewaf

FROM alpine:latest@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf
LABEL org.opencontainers.image.title="HomeWaf"
LABEL org.opencontainers.image.licenses="LicenseRef-HomeWaf-NC-1.0"
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata iptables
COPY --from=backend /out/homewaf /app/homewaf
COPY LICENSE NOTICE THIRD_PARTY_NOTICES.md /app/
COPY LICENSES /app/LICENSES
COPY docs/modernization/MIGRATION.md docs/modernization/SECURITY.md docs/modernization/provenance.json /app/docs/modernization/
VOLUME ["/app/conf", "/app/data", "/app/.homewaf-keys", "/app/logs", "/app/ssl"]
EXPOSE 26666 80 443 443/udp
ENTRYPOINT ["/app/homewaf"]
