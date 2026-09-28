#!/bin/sh
if [ "$1" = "0" ]; then
  systemctl --global disable lgrassconf.service || true
  rm -f /usr/lib/systemd/user/lgrassconf.service
  rm -f /etc/systemd/user/default.target.wants/lgrassconf.service
fi
