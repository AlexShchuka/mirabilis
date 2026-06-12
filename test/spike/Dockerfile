FROM node:26-trixie-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl python3 python3-venv \
    && rm -rf /var/lib/apt/lists/*

ARG CLAUDE_VERSION=2.1.175
RUN npm install -g @anthropic-ai/claude-code@${CLAUDE_VERSION}

RUN python3 -m venv /opt/headroom \
    && /opt/headroom/bin/pip install --no-cache-dir "headroom-ai[proxy]"

COPY entry.sh /entry.sh
RUN chmod 0755 /entry.sh

USER node
WORKDIR /home/node

ENTRYPOINT ["/entry.sh"]
