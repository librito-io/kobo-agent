#!/bin/sh
# udev-invoked launcher. Install to /usr/local/dropbear/boot.sh (rootfs).
#
# Renice neutral, then detach from udev (setsid + clean env) so udev cannot reap
# dropbear when the rule's RUN context exits. Without the setsid detach, dropbear
# dies the moment udev finishes processing the loop0 event.
renice 0 -p $$ >/dev/null 2>&1
env -i -- setsid /usr/local/dropbear/on-boot.sh &
exit 0
