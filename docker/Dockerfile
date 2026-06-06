FROM node:24-trixie-slim

ARG TZ=Etc/UTC
ENV TZ="$TZ" \
    DEBIAN_FRONTEND=noninteractive \
    DISABLE_AUTOUPDATER=1 \
    LANG=C.UTF-8

RUN rm -f /etc/apt/apt.conf.d/docker-clean \
    && echo 'Binary::apt::APT::Keep-Downloaded-Packages "true";' > /etc/apt/apt.conf.d/keep-cache

RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl git jq less sudo tini \
      bubblewrap socat \
      ripgrep unzip tzdata

RUN arch="$(dpkg --print-architecture)" \
    && case "$arch" in \
         arm64) target=aarch64-unknown-linux-gnu ;; \
         amd64) target=x86_64-unknown-linux-musl ;; \
         *) echo "unsupported arch: $arch" >&2; exit 1 ;; \
       esac \
    && base="https://github.com/rtk-ai/rtk/releases/latest/download" \
    && curl -fsSL -o "/tmp/rtk-${target}.tar.gz" "${base}/rtk-${target}.tar.gz" \
    && curl -fsSL -o /tmp/rtk-checksums.txt "${base}/checksums.txt" \
    && (cd /tmp && sha256sum --ignore-missing -c rtk-checksums.txt) \
    && tar -xzf "/tmp/rtk-${target}.tar.gz" -C /usr/local/bin rtk \
    && rm -f "/tmp/rtk-${target}.tar.gz" /tmp/rtk-checksums.txt \
    && chmod 0755 /usr/local/bin/rtk

RUN npm install -g ccstatusline@2.2.19

RUN mkdir -p /workspace /opt/mirabilis /home/node/.claude /home/node/.config/gh \
    && chown -R node:node /workspace /opt/mirabilis /home/node

COPY config/                    /opt/mirabilis/config/
COPY .claude-plugin/            /opt/mirabilis/marketplace/.claude-plugin/
COPY scripts/provision-mcp.sh   /usr/local/bin/provision-mcp.sh
COPY scripts/protect-critical.sh /opt/mirabilis/protect-critical.sh
COPY .devcontainer/refresh.sh    /opt/mirabilis/refresh.sh
RUN chmod 0755 /usr/local/bin/provision-mcp.sh /opt/mirabilis/protect-critical.sh /opt/mirabilis/refresh.sh

RUN git config --system url."https://github.com/".insteadOf "git@github.com:" \
    && git config --system --add url."https://github.com/".insteadOf "ssh://git@github.com/"

ARG MIRABILIS_VERSION=unknown
ENV MIRABILIS_VERSION=$MIRABILIS_VERSION

USER node
WORKDIR /workspace
