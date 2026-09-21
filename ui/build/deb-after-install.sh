#!/bin/sh
set -e
case "$(uname -m)" in
  aarch64) arch=arm64 ;;
  *) arch=amd64 ;;
esac
mkdir -p /usr/lib/systemd/user
cat > /usr/lib/systemd/user/lgconf.service <<UNIT
[Unit]
Description=lemongrass Claude Code config keeper

[Service]
Type=simple
ExecStart=/opt/Lemongrass/resources/bin/linux-$arch/lgconf run
Restart=on-failure
RestartSec=2

[Install]
WantedBy=default.target
UNIT
chmod 0755 /opt/Lemongrass/resources/bin/linux-$arch/lgconf || true
systemctl --global enable lgconf.service || true
