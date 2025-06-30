#!/usr/bin/env bash

# Re-run with the correct interpreter depending on the OS
# Use SKIP_SHEBANG_CHECK variable to prevent infinite loop if already re-run
# Get machine architecture
ARCH=$(uname -m)

if [[ "$ARCH" == "x86_64" ]] || [[ "$ARCH" == "amd64" ]]; then
    ARCH="amd64"
elif [[ "$ARCH" == "aarch64" || "$ARCH" == "arm64" ]]; then
    ARCH="arm64"
elif [[ "$ARCH" == "armv6l" ]]; then
    ARCH="armv6l"
elif [[ "$ARCH" == "i386" ]]; then
    ARCH="386"
else
    echo "$ARCH is an unsupported architecture."
    exit 1
fi

OS_TYPE=$(uname)

TOTAL_STEPS=12
STEP=1
SUCCESS="false"

# Detect current shell
CURRENT_SHELL=$(ps -p $$ -o comm=)

# Check Shell
SHELL_NAME=$(basename "$SHELL")
if [[ "$SHELL_NAME" == "zsh" ]]; then
    echo "The current shell is $SHELL_NAME. The installation will proceed based on $SHELL_NAME."
elif [[ "$SHELL_NAME" == "bash" ]]; then
    echo "The current shell is $SHELL_NAME. The installation will proceed based on $SHELL_NAME."
else
    echo "The current shell is $SHELL_NAME. $SHELL_NAME is an unsupported shell."
    exit 1
fi

# Set Config File
if [ "$SHELL_NAME" = "zsh" ]; then
    CONFIG_FILE="$HOME/.zshrc"
    PROFILE_FILE="$HOME/.zshrc"
elif [ "$SHELL_NAME" = "bash" ]; then
    CONFIG_FILE="$HOME/.bashrc"
    PROFILE_FILE="$HOME/.profile"
fi

# Function to display completion message
function display_completion_message {
    if [[ "$SUCCESS" == "true" ]]; then
        echo ""
        echo "All steps are complete."
        echo ""
        exit 0
    else
        echo ""
        echo "Installation was interrupted. Completed $((STEP - 1)) steps."
        echo ""
        echo "Please source your profile to apply changes:"
        echo -e "\033[1;32msource $CONFIG_FILE\033[0m"
        exit 1
    fi
}

# Use trap to display message on script exit, whether successful or due to an error
trap display_completion_message EXIT
trap "echo 'Process interrupted!'; exit 1" INT

if ! command -v sudo &> /dev/null; then
    echo "sudo not found, installing..."
    apt-get install -y sudo
fi

# 1. Update package list
echo "[$STEP/$TOTAL_STEPS] Updating package list..."
sudo apt-get update -y
STEP=$((STEP + 1))
echo

# 2. Install Build-essential
echo "[$STEP/$TOTAL_STEPS] Installing Build-essential..."
if ! dpkg -s build-essential &> /dev/null; then
    echo "Build-essential not found, installing..."
    sudo apt-get install -y build-essential
else
    echo "Build-essential is already installed."
fi
STEP=$((STEP + 1))
echo

# 3. Install Git
echo "[$STEP/$TOTAL_STEPS] Installing Git..."
if ! command -v git &> /dev/null; then
    echo "git not found, installing..."
    sudo apt-get install -y git
else
    echo "git is already installed."
fi
STEP=$((STEP + 1))
echo

# 4. Install Terraform
echo "[$STEP/$TOTAL_STEPS] Installing Terraform..."
if command -v terraform &> /dev/null && current_version=$(terraform --version | grep -oP "v\K[0-9]+\.[0-9]+") && (( $(echo "$current_version >= 1.1" | bc -l) )); then
    echo "Terraform v$current_version is already installed"
else
    echo "Installing Terraform..."
    sudo apt-get install -y gnupg software-properties-common curl
    curl -fsSL https://apt.releases.hashicorp.com/gpg | gpg --dearmor | sudo tee /usr/share/keyrings/hashicorp-archive-keyring.gpg > /dev/null
    echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/hashicorp.list
    sudo apt-get update && sudo apt-get install -y terraform
fi
STEP=$((STEP + 1))
echo

# 5. Install AWS CLI
echo "[$STEP/$TOTAL_STEPS] Installing AWS CLI..."
if command -v aws &> /dev/null && version=$(aws --version | cut -d/ -f2 | cut -d' ' -f1) && [[ $version == 2* ]]; then
    echo "AWS CLI v2 is already installed (version $version)"
else
    echo "Installing AWS CLI v2..."
    if ! command -v unzip &> /dev/null; then
        sudo apt-get install -y unzip
    fi
    if [ "$ARCH" = "arm64" ]; then
        curl "https://awscli.amazonaws.com/awscli-exe-linux-aarch64.zip" -o "awscliv2.zip"
    else
        curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
    fi
    unzip awscliv2.zip
    sudo ./aws/install --bin-dir /usr/local/bin --install-dir /usr/local/aws-cli --update
    rm -rf aws awscliv2.zip
fi
STEP=$((STEP + 1))
echo

# 6. Install Helm
echo "[$STEP/$TOTAL_STEPS] Installing Helm..."
if command -v helm &> /dev/null; then
    echo "Helm is already installed"
else
    echo "Installing Helm..."
    curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
    chmod 700 get_helm.sh
    ./get_helm.sh
    rm get_helm.sh
fi
STEP=$((STEP + 1))
echo

# 7. Install kubectl
echo "[$STEP/$TOTAL_STEPS] Installing kubectl..."
if command -v kubectl &> /dev/null; then
    echo "kubectl is already installed"
else
    echo "Installing kubectl..."
    if [[ "$ARCH" == "arm64" ]]; then
        curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/arm64/kubectl"
        curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/arm64/kubectl.sha256"
    else
        curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
        curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl.sha256"
    fi
    if echo "$(cat kubectl.sha256)  kubectl" | sha256sum --check; then
        sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
        rm kubectl kubectl.sha256
    else
        echo "kubectl checksum validation failed"
        rm kubectl kubectl.sha256
        exit 1
    fi
fi
STEP=$((STEP + 1))
echo

# 8. Install Node.js
echo "[$STEP/$TOTAL_STEPS] Installing Node.js (v20.16.0)..."
current_node_version=$(node -v 2>/dev/null)
if [[ "$current_node_version" != "v20.16.0" ]]; then
    export NVM_DIR="$HOME/.nvm"
    mkdir -p "$NVM_DIR"
    if ! command -v nvm &> /dev/null; then
        echo "Installing NVM..."
        curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash
        if ! grep -Fxq 'export NVM_DIR="$HOME/.nvm"' "$CONFIG_FILE"; then
            {
                echo ''
                echo 'export NVM_DIR="$HOME/.nvm"'
                echo "[ -s \"$NVM_DIR/nvm.sh\" ] && \. \"$NVM_DIR/nvm.sh\""
                echo "[ -s \"$NVM_DIR/bash_completion\" ] && \. \"$NVM_DIR/bash_completion\""
            } >> "$CONFIG_FILE"
        fi
        if ! grep -Fxq 'export NVM_DIR="$HOME/.nvm"' "$PROFILE_FILE"; then
            {
                echo ''
                echo 'export NVM_DIR="$HOME/.nvm"'
                echo "[ -s \"$NVM_DIR/nvm.sh\" ] && \. \"$NVM_DIR/nvm.sh\""
                echo "[ -s \"$NVM_DIR/bash_completion\" ] && \. \"$NVM_DIR/bash_completion\""
            } >> "$PROFILE_FILE"
        fi
    fi
    [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
    nvm install v20.16.0
    nvm use v20.16.0
    nvm alias default v20.16.0
else
    echo "Node.js v20.16.0 is already installed."
fi
STEP=$((STEP + 1))
echo

# 9. Install Pnpm
echo "[$STEP/$TOTAL_STEPS] Installing Pnpm..."
export PATH="$HOME/.local/share/pnpm:$PATH"
if ! command -v pnpm &> /dev/null; then
    echo "Installing pnpm..."
    curl -fsSL https://get.pnpm.io/install.sh | bash -
    if ! grep -Fq 'export PATH="$HOME/.local/share/pnpm:$PATH"' "$CONFIG_FILE"; then
        {
            echo ''
            echo 'export PATH="$HOME/.local/share/pnpm:$PATH"'
        } >> "$CONFIG_FILE"
    fi
    if ! grep -Fq 'export PATH="$HOME/.local/share/pnpm:$PATH"' "$PROFILE_FILE"; then
        {
            echo ''
            echo 'export PATH="$HOME/.local/share/pnpm:$PATH"'
        } >> "$PROFILE_FILE"
    fi
else
    echo "pnpm is already installed."
fi
STEP=$((STEP + 1))
echo

# 10. Verify npx availability
echo "[$STEP/$TOTAL_STEPS] Verifying npx availability..."
# Add npm global bin to PATH for npx access
export PATH="$PATH:$(npm config get prefix)/bin"
if command -v npx &> /dev/null; then
    echo "✅ npx is available and ready to use"
    npx --version
else
    echo "❌ npx is not available. This should not happen as npx comes with npm."
    echo "Attempting to install npx separately..."
    npm install -g npx
    if command -v npx &> /dev/null; then
        echo "✅ npx has been installed successfully"
        npx --version
    else
        echo "❌ Failed to install npx"
        exit 1
    fi
fi

# Add npm global bin to PATH in config files if not already present
NPM_GLOBAL_BIN="$(npm config get prefix)/bin"
if ! grep -Fq "export PATH=\"\$PATH:$NPM_GLOBAL_BIN\"" "$CONFIG_FILE"; then
    {
        echo ''
        echo "export PATH=\"\$PATH:$NPM_GLOBAL_BIN\""
    } >> "$CONFIG_FILE"
fi

if ! grep -Fq "export PATH=\"\$PATH:$NPM_GLOBAL_BIN\"" "$PROFILE_FILE"; then
    {
        echo ''
        echo "export PATH=\"\$PATH:$NPM_GLOBAL_BIN\""
    } >> "$PROFILE_FILE"
fi

STEP=$((STEP + 1))
echo

# 11. Install Foundry
echo "[$STEP/$TOTAL_STEPS] Installing Foundry..."
# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo "jq not found, installing..."
    sudo apt-get install -y jq
else
    echo "✅ jq is already installed"
fi

# Check if Foundry is already installed with expected version
if forge --version &> /dev/null && cast --version &> /dev/null; then
    echo "✅ Foundry is already installed"
else
    # Install Foundry
    echo "Installing/updating Foundry..."
    if ! command -v curl &> /dev/null; then
        echo "curl not found, installing..."
        sudo apt-get install -y curl
    fi
    # Install foundryup if not already installed
    if ! command -v foundryup &> /dev/null; then
        echo "Installing foundryup..."
        curl -L https://foundry.paradigm.xyz | bash
        export PATH="$HOME/.foundry/bin:$PATH"
        source $CONFIG_FILE
    fi
    # Install stable version of Foundry
    if foundryup --install stable; then
        echo "✅ Foundry has been installed successfully!"
        forge --version
        cast --version 
        anvil --version
    else
        echo "❌ Foundry installation failed"
        exit 1
    fi
fi

STEP=$((STEP + 1))
echo

# 12. Install Go
echo "[$STEP/$TOTAL_STEPS] Installing Go (v1.22.6)..."
export PATH="$PATH:/usr/local/go/bin"

# Save the current Go version
current_go_version=$(go version 2>/dev/null)

# Check if the current version is not v1.22.6
if ! echo "$current_go_version" | grep 'go1.22.6' &>/dev/null ; then
    echo "Installing go1.22.6..."
    # If Go is installed, remove it
    if command -v go &> /dev/null; then
        echo "Go is already installed. Removing the existing version..."
        sudo rm -rf "$(which go)"
    fi

    if ! command -v curl &> /dev/null; then
        echo "curl not found, installing..."
        sudo apt-get install -y curl
    else
        echo "curl is already installed."
    fi

    GO_FILE_NAME="go1.22.6.linux-${ARCH}.tar.gz"
    GO_DOWNLOAD_URL="https://go.dev/dl/${GO_FILE_NAME}"

    sudo curl -L -o "${GO_FILE_NAME}" "${GO_DOWNLOAD_URL}"

    sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf "${GO_FILE_NAME}"

    # Check if the Go configuration is already in the CONFIG_FILE
    if ! grep -Fxq 'export PATH="$PATH:/usr/local/go/bin"' "$CONFIG_FILE"; then
        # If the configuration is not found, add Go to the current shell session
        {
            echo ''
            echo 'export PATH="$PATH:/usr/local/go/bin"'
        } >> "$CONFIG_FILE"
    fi

    # Check if the Go configuration is already in the PROFILE_FILE
    if ! grep -Fxq 'export PATH=$PATH:/usr/local/go/bin' "$PROFILE_FILE"; then
        # If the configuration is not found, add Go to the current shell session
        {
            echo ''
            echo 'export PATH="$PATH:/usr/local/go/bin"'
        } >> "$PROFILE_FILE"
    fi

    rm -rf "${GO_FILE_NAME}"

    export PATH="$PATH:/usr/local/go/bin"
else
    echo "Go 1.22.6 is already installed."
fi

# Add required PATH exports if not already present
if ! grep -q "export PATH=\$PATH:/usr/local/go/bin" "$CONFIG_FILE"; then
    echo "export PATH=\$PATH:/usr/local/go/bin" >> "$CONFIG_FILE"
fi

if ! grep -q "export PATH=\$HOME/go/bin:\$PATH" "$CONFIG_FILE"; then
    echo "export PATH=\$HOME/go/bin:\$PATH" >> "$CONFIG_FILE"
fi

STEP=$((STEP + 1))
echo

SUCCESS="true"

# Function to check if a command exists and its version if necessary
function check_command_version {
    CMD=$1
    EXPECTED_VERSION=$2
    VERSION_CMD=$3

    if command -v "$CMD" &> /dev/null; then
        CURRENT_VERSION=$(eval $VERSION_CMD 2>&1 | head -n 1)

        if [[ -z "$EXPECTED_VERSION" ]]; then
            if [[ "$CMD" == "forge" || "$CMD" == "cast" || "$CMD" == "anvil" ]]; then
                echo "✅ foundry - $CMD is installed. Current version: $CURRENT_VERSION"
            else
                echo "✅ $CMD is installed. Current version: $CURRENT_VERSION"
            fi
        elif echo "$CURRENT_VERSION" | grep -q "$EXPECTED_VERSION"; then
            echo "✅ $CMD is installed and matches version $EXPECTED_VERSION."
        else
            echo "❌ $CMD is installed but version does not match $EXPECTED_VERSION. Current version: $CURRENT_VERSION"
        fi
    else
        if [[ "$CMD" == "forge" || "$CMD" == "cast" || "$CMD" == "anvil" ]]; then
            echo "❌ foundry - $CMD is not installed."
        else
            echo "❌ $CMD is not installed."
        fi
    fi
}

if [[ "$SUCCESS" == "true" ]]; then
    echo "All required tools are installed and ready to use!"
else
    echo "Some tools failed to install. Please check the output above for details."
    exit 1
fi

check_command_version git "" "git --version"
check_command_version make "" "make --version"
check_command_version gcc "" "gcc --version"
check_command_version node "v20.16.0" "node -v"
check_command_version pnpm "" "pnpm --version"
check_command_version npx "" "npx --version"
check_command_version terraform "" "terraform --version"
check_command_version aws "" "aws --version"
check_command_version helm "" "helm version"
check_command_version kubectl "" "kubectl version --client"
check_command_version forge "" "forge --version"
check_command_version cast "" "cast --version"
check_command_version anvil "" "anvil --version"
check_command_version go "" "go version"

echo "🎉 All required tools are installed and ready to use!" 