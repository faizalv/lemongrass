#!/bin/sh
systemctl --global disable lgrassconf.service || true
rm -f /usr/lib/systemd/user/lgrassconf.service
systemctl --global disable lgconf.service || true
rm -f /usr/lib/systemd/user/lgconf.service
