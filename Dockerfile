# Build frontend
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# Build Go binary
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist /tmp/webdist
RUN rm -rf cmd/app/static && mkdir -p cmd/app/static && cp -a /tmp/webdist/. cmd/app/static/
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/app ./cmd/app

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/app /app/app
COPY configs /app/configs
COPY migrations /app/migrations
ENV TZ=Europe/Moscow
EXPOSE 8080
ENTRYPOINT ["/app/app", "-config", "/app/configs/config.yaml"]
