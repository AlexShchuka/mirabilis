FROM golang:1.26-trixie@sha256:68b7145ec43d1820b9a56704554b53d1520aa2a15cb5233e374188a31b2a1bce AS build

ARG MIRABILIS_VERSION=unknown

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags "-X main.version=${MIRABILIS_VERSION}" -o /out/mirabilis ./cmd/mirabilis

FROM node:26-trixie-slim@sha256:a1d9d671994fc2d26e297ac56b4b1522a8bc7fa71c43b14cd1b1fe6c5116f7dc

ARG TZ=Etc/UTC
ENV TZ="$TZ" \
    DEBIAN_FRONTEND=noninteractive \
    DISABLE_AUTOUPDATER=1 \
    LANG=C.UTF-8

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN rm -f /etc/apt/apt.conf.d/docker-clean \
    && echo 'Binary::apt::APT::Keep-Downloaded-Packages "true";' > /etc/apt/apt.conf.d/keep-cache

RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl git jq less sudo tini \
      ripgrep unzip tzdata \
      gcc libc6-dev \
      python3 python3-venv python3-pip

ARG RTK_VERSION=v0.42.3
RUN arch="$(dpkg --print-architecture)" \
    && case "$arch" in \
         arm64) target=aarch64-unknown-linux-gnu ;; \
         amd64) target=x86_64-unknown-linux-musl ;; \
         *) echo "unsupported arch: $arch" >&2; exit 1 ;; \
       esac \
    && base="https://github.com/rtk-ai/rtk/releases/download/${RTK_VERSION}" \
    && curl --retry 3 --retry-delay 2 --retry-all-errors -fsSL -o "/tmp/rtk-${target}.tar.gz" "${base}/rtk-${target}.tar.gz" \
    && curl --retry 3 --retry-delay 2 --retry-all-errors -fsSL -o /tmp/rtk-checksums.txt "${base}/checksums.txt" \
    && (cd /tmp && sha256sum --ignore-missing -c rtk-checksums.txt) \
    && tar -xzf "/tmp/rtk-${target}.tar.gz" -C /usr/local/bin rtk \
    && rm -f "/tmp/rtk-${target}.tar.gz" /tmp/rtk-checksums.txt \
    && chmod 0755 /usr/local/bin/rtk

ARG UV_VERSION=0.11.19
RUN curl --retry 3 --retry-delay 2 --retry-all-errors -fsSL "https://astral.sh/uv/${UV_VERSION}/install.sh" \
    | env UV_INSTALL_DIR=/usr/local/bin UV_NO_MODIFY_PATH=1 sh

ARG STACKS=
ARG DOTNET_CHANNEL=10.0
ENV DOTNET_ROOT=/usr/share/dotnet
ENV PATH=$PATH:/usr/share/dotnet
RUN case ",${STACKS}," in \
      *,dotnet,*) \
        curl --retry 3 --retry-delay 2 --retry-all-errors -fsSL https://dot.net/v1/dotnet-install.sh -o /tmp/dotnet-install.sh \
        && chmod +x /tmp/dotnet-install.sh \
        && /tmp/dotnet-install.sh --channel "$DOTNET_CHANNEL" --install-dir /usr/share/dotnet \
        && ln -sf /usr/share/dotnet/dotnet /usr/local/bin/dotnet \
        && rm -f /tmp/dotnet-install.sh ;; \
    esac

ARG GO_VERSION=go1.25.8
ENV GOROOT=/usr/local/go
ENV GOPATH=/home/node/go
ENV PATH=$PATH:/usr/local/go/bin:/home/node/go/bin
RUN arch="$(dpkg --print-architecture)" \
    && case "$arch" in \
         arm64) goarch=arm64 ;; \
         amd64) goarch=amd64 ;; \
         *) echo "unsupported arch: $arch" >&2; exit 1 ;; \
       esac \
    && tarball="${GO_VERSION}.linux-${goarch}.tar.gz" \
    && curl --retry 3 --retry-delay 2 --retry-all-errors -fsSL -o /tmp/go.tar.gz "https://dl.google.com/go/${tarball}" \
    && expected="$(curl --retry 3 --retry-delay 2 --retry-all-errors -fsSL "https://dl.google.com/go/${tarball}.sha256")" \
    && echo "${expected}  /tmp/go.tar.gz" | sha256sum -c - \
    && rm -rf /usr/local/go \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm -f /tmp/go.tar.gz

ARG GITLEAKS_VERSION=v8.30.1
ARG GOLANGCI_LINT_VERSION=v2.12.2
ARG GO_ARCH_LINT_VERSION=v1.15.0
WORKDIR /tmp/gomod-tools
COPY go.mod go.sum ./
RUN go install tool \
    && go install github.com/zricethezav/gitleaks/v8@${GITLEAKS_VERSION} \
    && go install github.com/fe3dback/go-arch-lint@${GO_ARCH_LINT_VERSION} \
    && go clean -modcache \
    && rm -rf /root/.cache/go-build /tmp/gomod-tools

RUN curl --retry 3 --retry-delay 2 --retry-all-errors -fsSL https://golangci-lint.run/install.sh | sh -s -- -b /home/node/go/bin ${GOLANGCI_LINT_VERSION}

ARG CLAUDE_CODE_VERSION=2.1.177
ARG GH_VERSION=2.95.0
ARG DOCKER_CE_CLI_VERSION=5:28.1.1-1~debian.13~trixie
RUN install -m 0755 -d /etc/apt/keyrings \
    && curl --retry 3 --retry-delay 2 --retry-all-errors -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
       -o /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && chmod a+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
       > /etc/apt/sources.list.d/github-cli.list \
    && curl --retry 3 --retry-delay 2 --retry-all-errors -fsSL https://download.docker.com/linux/debian/gpg \
       -o /etc/apt/keyrings/docker.gpg \
    && chmod a+r /etc/apt/keyrings/docker.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian trixie stable" \
       > /etc/apt/sources.list.d/docker.list
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update \
    && apt-get install -y --no-install-recommends \
         "gh=${GH_VERSION}" \
         "docker-ce-cli=${DOCKER_CE_CLI_VERSION}" \
    && rm -f /etc/apt/sources.list.d/github-cli.list \
             /etc/apt/sources.list.d/docker.list \
             /etc/apt/keyrings/githubcli-archive-keyring.gpg \
             /etc/apt/keyrings/docker.gpg

ARG BATS_VERSION=1.13.0
RUN npm install -g @anthropic-ai/claude-code@${CLAUDE_CODE_VERSION} \
    && npm install -g bats@${BATS_VERSION} \
    && rm -rf /root/.npm

ENV GOTMPDIR=/home/node/.cache/go-tmp

ENV MIRABILIS_STACKS=$STACKS
ENV MIRABILIS_REPO=/opt/mirabilis

RUN mkdir -p /workspace /opt/mirabilis /home/node/.claude /home/node/.config/gh /home/node/.cache/go-tmp \
    && chown -R node:node /workspace /opt/mirabilis /home/node

COPY config/                    /opt/mirabilis/config/
COPY --from=build /out/mirabilis /usr/local/bin/mirabilis
RUN chmod 0755 /usr/local/bin/mirabilis

RUN git config --system url."https://github.com/".insteadOf "git@github.com:" \
    && git config --system --add url."https://github.com/".insteadOf "ssh://git@github.com/"

ARG MIRABILIS_VERSION=unknown
ENV MIRABILIS_VERSION=$MIRABILIS_VERSION

RUN printf 'node ALL=(ALL) NOPASSWD: ALL\n' > /etc/sudoers.d/node \
    && chmod 0440 /etc/sudoers.d/node \
    && visudo -cf /etc/sudoers.d/node

USER node
WORKDIR /workspace
