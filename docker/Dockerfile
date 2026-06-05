FROM debian:bookworm-slim

ARG CLAUDE_CODE_VERSION=2.1.89
ARG INSTALL_NODE=false
ARG CODER_UID=1000
ARG CODER_GID=1000

ENV DEBIAN_FRONTEND=noninteractive \
    DISABLE_AUTOUPDATER=1 \
    LANG=C.UTF-8 \
    PATH=/home/coder/.local/bin:/usr/local/bin:/usr/bin:/bin

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl git jq less sudo gosu tini \
      iptables ipset iproute2 dnsutils \
      ripgrep unzip tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && update-alternatives --set iptables  /usr/sbin/iptables-legacy  || true \
    && update-alternatives --set ip6tables /usr/sbin/ip6tables-legacy || true

RUN install -m 0755 -d /etc/apt/keyrings \
    && curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
         -o /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
         > /etc/apt/sources.list.d/github-cli.list \
    && apt-get update && apt-get install -y --no-install-recommends gh \
    && rm -rf /var/lib/apt/lists/*

RUN if [ "$INSTALL_NODE" = "true" ]; then \
      curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
      && apt-get install -y --no-install-recommends nodejs \
      && rm -rf /var/lib/apt/lists/* ; \
    fi

RUN groupadd -g "$CODER_GID" coder \
    && useradd -m -u "$CODER_UID" -g "$CODER_GID" -s /bin/bash coder
RUN printf 'coder ALL=(root) NOPASSWD: /usr/local/bin/init-firewall.sh\n' \
      > /etc/sudoers.d/coder-firewall && chmod 0440 /etc/sudoers.d/coder-firewall

USER coder
RUN curl -fsSL https://claude.ai/install.sh | bash -s -- "$CLAUDE_CODE_VERSION" \
    && /home/coder/.local/bin/claude --version
USER root

COPY docker/init-firewall.sh  /usr/local/bin/init-firewall.sh
COPY docker/entrypoint.sh     /usr/local/bin/entrypoint.sh
COPY scripts/provision-mcp.sh /usr/local/bin/provision-mcp.sh
COPY config/                  /opt/mirabilis/config/
COPY .claude-plugin/          /opt/mirabilis/marketplace/.claude-plugin/
RUN chmod 0755 /usr/local/bin/init-firewall.sh \
               /usr/local/bin/entrypoint.sh \
               /usr/local/bin/provision-mcp.sh

WORKDIR /workspace
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/entrypoint.sh"]
CMD ["sleep", "infinity"]
