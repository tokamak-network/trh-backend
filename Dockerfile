FROM golang:1.24.11 AS builder

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
ENV SHELL=/bin/bash

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

# Set environment variables for tools (will be installed via setup.sh)
ENV NVM_DIR=/root/.nvm
ENV PNPM_HOME=/root/.local/share/pnpm
ENV PATH="/root/.local/share/pnpm:/root/.nvm/versions/node/v20.16.0/bin:/root/.foundry/bin:/usr/local/go/bin:/root/go/bin:/usr/local/bin:${PATH}"

# Copy Go SDK from builder stage (needed for op-program build during L2 deployment)
COPY --from=builder /usr/local/go /usr/local/go

# Install Node.js v20.16.0 via nvm
RUN curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash \
    && . "$NVM_DIR/nvm.sh" \
    && nvm install 20.16.0 \
    && nvm use 20.16.0 \
    && nvm alias default 20.16.0

# Install pnpm via npm (installs to nvm node bin directory)
RUN . "$NVM_DIR/nvm.sh" \
    && npm install -g pnpm

# Install Foundry (forge, cast, anvil)
RUN curl -L https://foundry.paradigm.xyz | bash \
    && /root/.foundry/bin/foundryup

# Create symlinks for tools in /usr/local/bin
RUN ln -sf /root/.nvm/versions/node/v20.16.0/bin/node /usr/local/bin/node \
    && ln -sf /root/.nvm/versions/node/v20.16.0/bin/npm /usr/local/bin/npm \
    && ln -sf /root/.nvm/versions/node/v20.16.0/bin/npx /usr/local/bin/npx \
    && ln -sf /root/.nvm/versions/node/v20.16.0/bin/pnpm /usr/local/bin/pnpm \
    && ln -sf /root/.foundry/bin/forge /usr/local/bin/forge \
    && ln -sf /root/.foundry/bin/cast /usr/local/bin/cast \
    && ln -sf /root/.foundry/bin/anvil /usr/local/bin/anvil

# Verify installations
RUN go version \
    && node --version \
    && pnpm --version \
    && forge --version \
    && cast --version \
    && anvil --version

WORKDIR /app

COPY --from=builder /app/main .

EXPOSE 8000

CMD ["./main"]
