FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o synthetic-monitor .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /app/synthetic-monitor /synthetic-monitor
USER 65532:65532
ENTRYPOINT ["/synthetic-monitor"]
