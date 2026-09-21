#!/bin/sh
systemctl --global disable lgconf.service || true
rm -f /usr/lib/systemd/user/lgconf.service
