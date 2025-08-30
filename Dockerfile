# Stage 1: Builder
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev  # CGO 支持

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o linkwatch cmd/main.go  # 构建 cmd/main.go

# Stage 2: Runtime
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/linkwatch .

CMD ["./linkwatch"]