# Luna sandbox image
# A practical dev environment for AI-assisted coding inside a Docker sandbox.
# Network is disabled at runtime (--network none), so all tools must be pre-installed.
#
# Build:
#   docker build -t luna-sandbox:latest scripts/sandbox/
#
# Use with luna:
#   luna --sandbox --sandbox-image luna-sandbox:latest "..."

FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive \
    LANG=C.UTF-8 \
    LC_ALL=C.UTF-8

# Core tools + Python + Node.js
RUN apt-get update && apt-get install -y --no-install-recommends \
    # Shell essentials
    bash \
    curl \
    wget \
    git \
    make \
    jq \
    unzip \
    zip \
    # Text processing
    ripgrep \
    # Python
    python3 \
    python3-pip \
    python3-venv \
    # Build tools
    gcc \
    build-essential \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Node.js 20 LTS (via nodesource — network required at build time)
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# uv (fast Python package manager)
RUN curl -LsSf https://astral.sh/uv/install.sh | sh \
    && ln -s /root/.cargo/bin/uv /usr/local/bin/uv 2>/dev/null || \
       ln -s /root/.local/bin/uv /usr/local/bin/uv 2>/dev/null || true

# Sanity check
RUN bash --version && python3 --version && node --version && pip3 --version

WORKDIR /workspace
