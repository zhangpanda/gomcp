FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /gomcp-server ./examples/basic/

FROM alpine:3.21
COPY --from=builder /gomcp-server /usr/local/bin/gomcp-server
ENTRYPOINT ["gomcp-server"]
