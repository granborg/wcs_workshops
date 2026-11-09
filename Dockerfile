# Builds the Vue app, embeds it in the Go API, and produces a small static image.
# Build from the repo root: docker build -t wcs-workshops .

# ---- web app ----
FROM node:22-alpine AS web
WORKDIR /src
COPY package.json package-lock.json ./
COPY apps/app/package.json apps/app/
RUN npm ci
COPY apps/app apps/app
# Firebase web config is public by design; pass it at build time (fly.toml [build.args]).
ARG VITE_FIREBASE_API_KEY
ARG VITE_FIREBASE_AUTH_DOMAIN
ARG VITE_FIREBASE_PROJECT_ID
ARG VITE_FIREBASE_APP_ID
ARG VITE_FIREBASE_MESSAGING_SENDER_ID
RUN npm run build -w apps/app

# ---- api ----
FROM golang:1.26-alpine AS api
WORKDIR /src/apps/api
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api .
COPY --from=web /src/apps/app/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# ---- runtime ----
# distroless/static ships CA certificates (needed for Vimeo and Google) and nothing else.
FROM gcr.io/distroless/static-debian12
COPY --from=api /out/api /api
ENV HOST=0.0.0.0 PORT=8080 DATABASE_PATH=/data/wcs.db
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/api"]
