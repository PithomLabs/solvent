# Multi-stage build for Solvent cloud demo
# Stage 1: Build the Go application
FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/
COPY kernel/ kernel/
COPY db/ db/
COPY demo/ demo/

RUN CGO_ENABLED=0 GOOS=linux go build -o /solvent-web ./demo/cloud/web/
RUN CGO_ENABLED=0 GOOS=linux go build -o /solvent-init ./demo/cloud/init/

# Stage 2: Runtime
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /solvent-web /app/solvent-web
COPY --from=builder /solvent-init /app/solvent-init
COPY --from=builder /app/db/ /app/db/
COPY --from=builder /app/demo/cloud/web/templates/ /app/demo/cloud/web/templates/
COPY --from=builder /app/internal/derive/testdata/ /app/internal/derive/testdata/

EXPOSE 8080

ENTRYPOINT ["/bin/sh", "-c", "/app/solvent-init; exec /app/solvent-web"]
