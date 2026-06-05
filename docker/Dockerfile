FROM node:20-bookworm-slim

ARG TZ=Etc/UTC
ENV TZ="$TZ" \
    DEBIAN_FRONTEND=noninteractive \
    DISABLE_AUTOUPDATER=1 \
    LANG=C.UTF-8

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl git jq less sudo tini \
      bubblewrap socat \
      ripgrep unzip tzdata \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /workspace /opt/mirabilis /home/node/.claude /home/node/.config/gh \
    && chown -R node:node /workspace /opt/mirabilis /home/node

COPY config/                    /opt/mirabilis/config/
COPY .claude-plugin/            /opt/mirabilis/marketplace/.claude-plugin/
COPY scripts/provision-mcp.sh   /usr/local/bin/provision-mcp.sh
COPY .devcontainer/refresh.sh    /opt/mirabilis/refresh.sh
RUN chmod 0755 /usr/local/bin/provision-mcp.sh /opt/mirabilis/refresh.sh

USER node
WORKDIR /workspace
