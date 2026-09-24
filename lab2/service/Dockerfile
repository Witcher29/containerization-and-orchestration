# ---------- Stage 1: Build ----------
FROM golang:1.22-alpine AS builder
    
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /api-service .

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /api-service /api-service

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/api-service"]