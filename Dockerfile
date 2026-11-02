# Stage 1: Build React + TypeScript Frontend
FROM node:22-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: Build Go Discrete-Event Simulation Server
FROM golang:1.22-alpine AS backend-builder
WORKDIR /app/backend
COPY backend/go.mod ./
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /failurelab-server ./cmd/server

# Stage 3: Minimal Production Runtime
FROM alpine:3.20
RUN apk --no-cache add ca-certificates && adduser -D -g '' appuser
WORKDIR /app
COPY --from=backend-builder /failurelab-server /app/failurelab-server
COPY --from=frontend-builder /app/frontend/dist /app/static

USER appuser
ENV PORT=8080
ENV STATIC_DIR=/app/static
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:${PORT}/healthz || exit 1

ENTRYPOINT ["/app/failurelab-server"]
