# ─── build ───────────────────────────────────
FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache module downloads
COPY go.mod go.sum ./
RUN go mod download

# Build (migrations are embedded via //go:embed, no separate copy needed)
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/vortex ./cmd/api

# ─── run ─────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/vortex /vortex
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/vortex"]
