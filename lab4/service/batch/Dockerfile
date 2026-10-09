FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY main.go .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /batch .

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /batch /batch

ENTRYPOINT ["/batch"]