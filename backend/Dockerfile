FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /app/server /server
USER 65532:65532
EXPOSE 8081
ENTRYPOINT ["/server"]
