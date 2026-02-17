# Build stage
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /cq-source-postgres .

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /cq-source-postgres /cq-source-postgres

EXPOSE 7777

ENTRYPOINT ["/cq-source-postgres", "serve", "--address", "[::]:7777"]
