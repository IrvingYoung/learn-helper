# syntax=docker/dockerfile:1.7

# ---- Frontend build ----
FROM node:20-alpine AS frontend
# Skip corepack (would fetch pnpm from network); install pnpm directly via npm
# with the China mirror so the binary and registry are both reachable.
RUN npm install -g pnpm@9 --registry=https://registry.npmmirror.com
ENV PNPM_REGISTRY=https://registry.npmmirror.com
WORKDIR /app
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY frontend/package.json frontend/
RUN pnpm install --frozen-lockfile
COPY frontend/ frontend/
RUN pnpm --filter learn-helper-frontend build

# ---- Go binary build ----
FROM golang:1.25-alpine AS backend
# goproxy.cn mirrors proxy.golang.org from China
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
COPY --from=frontend /app/frontend/dist/ ./cmd/server/dist/
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/learn-helper ./cmd/server

# ---- Runtime ----
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend /out/learn-helper /app/learn-helper
RUN mkdir -p /app/data
EXPOSE 8080
ENV PORT=8080
ENV DB_PATH=/app/data/learn-helper.db
VOLUME /app/data
ENTRYPOINT ["/app/learn-helper"]
