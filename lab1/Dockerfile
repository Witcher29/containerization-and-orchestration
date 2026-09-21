FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY api.go .
RUN CGO_ENABLED=0 go build -o api api.go

FROM scratch
COPY --from=builder /app/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
