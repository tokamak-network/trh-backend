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

# Set environment variables for tools
ENV NVM_DIR=/root/.nvm
ENV PNPM_HOME=/root/.local/share/pnpm
ENV PATH="/root/.local/share/pnpm:/root/.nvm/versions/node/v20.16.0/bin:/root/.foundry/bin:/usr/local/go/bin:/root/go/bin:/usr/local/bin:${PATH}"

# Install all dependencies (AWS CLI, Terraform, Helm, kubectl, Node.js, pnpm, Foundry, Go)
COPY docker_install_dependencies_script.sh /tmp/docker_install_dependencies_script.sh
RUN chmod +x /tmp/docker_install_dependencies_script.sh && \
    bash /tmp/docker_install_dependencies_script.sh && \
    rm /tmp/docker_install_dependencies_script.sh && \
    # Create symlinks for tools to be available in PATH
    ln -sf /root/.local/share/pnpm/pnpm /usr/local/bin/pnpm || true && \
    ln -sf /root/.nvm/versions/node/v20.16.0/bin/npx /usr/local/bin/npx || true && \
    ln -sf /root/.nvm/versions/node/v20.16.0/bin/node /usr/local/bin/node || true && \
    ln -sf /root/.nvm/versions/node/v20.16.0/bin/npm /usr/local/bin/npm || true && \
    ln -sf /root/.foundry/bin/forge /usr/local/bin/forge || true && \
    ln -sf /root/.foundry/bin/cast /usr/local/bin/cast || true && \
    ln -sf /root/.foundry/bin/anvil /usr/local/bin/anvil || true

WORKDIR /app

COPY --from=builder /app/main .

EXPOSE 8000

CMD ["./main"]
