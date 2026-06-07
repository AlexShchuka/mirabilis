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
      ripgrep unzip tzdata \
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

RUN npm install -g ccstatusline@2.2.19

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

RUN mkdir -p /workspace /opt/mirabilis /home/node/.claude /home/node/.config/gh \
    && chown -R node:node /workspace /opt/mirabilis /home/node

COPY config/                    /opt/mirabilis/config/
COPY .claude-plugin/            /opt/mirabilis/marketplace/.claude-plugin/
COPY src/provision-mcp.sh       /usr/local/bin/provision-mcp.sh
COPY src/git-identity.sh        /usr/local/bin/git-identity.sh
COPY .devcontainer/refresh.sh    /opt/mirabilis/refresh.sh
COPY .devcontainer/entrypoint.sh /opt/mirabilis/entrypoint.sh
RUN chmod 0755 /usr/local/bin/provision-mcp.sh /usr/local/bin/git-identity.sh /opt/mirabilis/refresh.sh /opt/mirabilis/entrypoint.sh

RUN git config --system url."https://github.com/".insteadOf "git@github.com:" \
    && git config --system --add url."https://github.com/".insteadOf "ssh://git@github.com/"

ARG MIRABILIS_VERSION=unknown
ENV MIRABILIS_VERSION=$MIRABILIS_VERSION

RUN printf 'node ALL=(ALL) NOPASSWD: ALL\n' > /etc/sudoers.d/node \
    && chmod 0440 /etc/sudoers.d/node \
    && visudo -cf /etc/sudoers.d/node

USER node
WORKDIR /workspace
