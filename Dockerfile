# syntax=docker/dockerfile:1.7

# ---- Frontend build ----
FROM node:20-alpine AS frontend
RUN corepack enable
WORKDIR /app
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY frontend/package.json frontend/
RUN pnpm install --frozen-lockfile
COPY frontend/ frontend/
RUN pnpm --filter learn-helper-frontend build

# ---- Go binary build ----
FROM golang:1.25-alpine AS backend
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
