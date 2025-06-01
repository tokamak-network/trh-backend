# Build Stage
FROM golang:1.22.6 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 🧊 Build a fully static binary (no glibc/musl required)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o main ./cmd/main.go

# Runtime Stage
FROM alpine:latest

WORKDIR /app

# Add required packages
RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/main .
COPY .env . 

EXPOSE 8080

CMD ["./main"]
