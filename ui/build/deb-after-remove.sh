#!/bin/sh
# SPDX-License-Identifier: MPL-2.0
# Copyright (C) 2026 The Lemongrass Authors

systemctl --global disable lgrassconf.service || true
rm -f /usr/lib/systemd/user/lgrassconf.service
systemctl --global disable lgconf.service || true
rm -f /usr/lib/systemd/user/lgconf.service
