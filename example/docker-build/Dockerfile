FROM ghcr.io/nxtcoder17/nix:latest AS builder
# RUN curl -L0 https://github.com/nxtcoder17/nixy/releases/download/v1.4.0/nixy-linux-amd64 > /usr/local/bin/nixy && chmod +x /usr/local/bin/nixy

RUN --mount=type=bind,source=nixy.yml,target=nixy.yml \
  --mount=type=bind,source=./nixy,target=/usr/local/bin/nixy \
  --mount=type=cache,target=/root/.local/share/nixy \
  --mount=type=cache,target=/nix \
  <<EOF
nixy shell echo "nixy setup complete"
curl -L0 https://google.com
exit 1
EOF

