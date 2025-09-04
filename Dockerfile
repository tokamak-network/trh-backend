FROM golang:1.22.6 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./main.go

FROM ubuntu:latest

# Set environment variables to prevent interactive prompts
ENV DEBIAN_FRONTEND=noninteractive
ENV TZ=UTC
ENV DEBCONF_NONINTERACTIVE_SEEN=true
ENV DEBCONF_NOWARNINGS=true

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    sudo \
    git \
    build-essential \
    curl \
    wget \
    ca-certificates \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

# Configure timezone non-interactively
RUN ln -sf /usr/share/zoneinfo/UTC /etc/localtime && \
    echo 'UTC' > /etc/timezone

WORKDIR /app

COPY --from=builder /app/main .

EXPOSE 8000

CMD ["./main"]
