# E2E test environment for usp.
# Fat container: all 4 CLI agents + usp + xrr + Go toolchain.
#
# Build:
#   docker build -f e2e/docker/Dockerfile -t usp-e2e .
#
# Run:
#   docker run --rm -it \
#     -e XRR_MODE=record \
#     -e XRR_CASSETTE_DIR=/cassettes \
#     -v ./e2e/docker/cassettes:/cassettes \
#     usp-e2e

FROM debian:bookworm-slim

# ── base prereqs ────────────────────────────────────────────────
RUN apt-get update -y \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y \
      --no-install-recommends \
      ca-certificates \
      curl \
      git \
      jq \
      sqlite3 \
 && rm -rf /var/lib/apt/lists/*

# ── Node.js 22.x LTS (for npm-based CLIs) ──────────────────────
RUN curl -fsSL https://deb.nodesource.com/setup_22.x \
      | bash - \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y \
      --no-install-recommends nodejs \
 && rm -rf /var/lib/apt/lists/*

# ── Go 1.26 (for building usp + xrr, and running go test) ──────
ARG GO_VERSION=1.26.2
RUN curl -fsSL \
      "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
      | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}"

# ── CLI version build args ──────────────────────────────────────
ARG CLAUDE_VERSION=latest
ARG CODEX_VERSION=latest
ARG GEMINI_VERSION=latest
ARG OPENCODE_VERSION=latest

# ── Claude Code (Anthropic) ─────────────────────────────────────
RUN npm install -g \
      "@anthropic-ai/claude-code@${CLAUDE_VERSION}"
RUN claude --version

# ── Codex (OpenAI) ──────────────────────────────────────────────
RUN npm install -g \
      "@openai/codex@${CODEX_VERSION}"
RUN codex --version

# ── Gemini CLI (Google) ─────────────────────────────────────────
RUN npm install -g \
      "@google/gemini-cli@${GEMINI_VERSION}"
RUN gemini --version

# ── OpenCode (sst) ──────────────────────────────────────────────
# Primary: official installer. Fallback: npm package.
RUN curl -fsSL https://opencode.ai/install | bash \
 && ln -sf /root/.opencode/bin/opencode /usr/local/bin/opencode \
      2>/dev/null \
 || npm install -g "opencode-ai@${OPENCODE_VERSION}"
RUN opencode --version

# ── Build usp from source ──────────────────────────────────────
WORKDIR /src/usp
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /usr/local/bin/usp ./cmd/usp
RUN usp --version

# ── Install xrr ────────────────────────────────────────────────
RUN go install hop.top/xrr/go/cmd/xrr@latest
RUN xrr --version

# ── Runtime env ─────────────────────────────────────────────────
ENV XRR_MODE="record"
ENV XRR_CASSETTE_DIR="/cassettes"

WORKDIR /src/usp

CMD ["go", "test", "-v", "-count=1", "-tags", "docker", "./e2e/docker/..."]
