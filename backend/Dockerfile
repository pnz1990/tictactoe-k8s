FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download || true
COPY main.go .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o server .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /app/server /server
USER 65532:65532
EXPOSE 8081
ENTRYPOINT ["/server"]
