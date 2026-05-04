#!/usr/bin/env bash
# One-time Waydroid + libndk_translation setup for testing arm64 wt APK on x86_64 Linux.
# Run with sudo. Idempotent.
#
#   sudo bash scripts/waydroid-setup.sh
#
# After this finishes, run (without sudo): scripts/waydroid-run.sh

set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
	echo "Re-running with sudo..." >&2
	exec sudo -E bash "$0" "$@"
fi

REAL_USER="${SUDO_USER:-$USER}"
REAL_HOME="$(getent passwd "$REAL_USER" | cut -d: -f6)"

log() { printf '\033[1;36m[waydroid-setup]\033[0m %s\n' "$*" >&2; }

# 1. Prereqs + official repo
log "Installing prerequisites and adding Waydroid repository"
apt-get update -qq
apt-get install -y curl ca-certificates python3 python3-pip git unzip lzip

if [ ! -f /etc/apt/sources.list.d/waydroid.list ] && [ ! -f /etc/apt/sources.list.d/waydroid.sources ]; then
	curl -s https://repo.waydro.id | bash
fi

# 2. Waydroid itself
log "Installing waydroid package"
apt-get install -y waydroid

# 3. Kernel module check (modern kernels have binder built in; ashmem replaced by memfd)
log "Checking kernel modules"
if ! lsmod | grep -q '^binder_linux'; then
	modprobe binder_linux || log "WARNING: failed to load binder_linux — Waydroid may not start"
fi

# 4. Init container with vanilla image (no GAPPS — we only test our APK)
if [ ! -f /var/lib/waydroid/waydroid.cfg ]; then
	log "Initializing Waydroid (downloading Android system image, ~500 MB)"
	waydroid init -s VANILLA -f
else
	log "Waydroid already initialized"
fi

# 5. Enable + start container service
log "Enabling waydroid-container service"
systemctl enable --now waydroid-container.service || true

# 6. Install libndk_translation (ARM-on-x86 translator extracted from Android Studio emulator)
log "Cloning waydroid_script and installing libndk translator"
SCRIPT_DIR=/opt/waydroid_script
if [ ! -d "$SCRIPT_DIR/.git" ]; then
	git clone --depth=1 https://github.com/casualsnek/waydroid_script "$SCRIPT_DIR"
else
	(cd "$SCRIPT_DIR" && git pull --ff-only) || true
fi

(cd "$SCRIPT_DIR" && python3 -m pip install --break-system-packages -r requirements.txt 2>/dev/null ||
	python3 -m pip install -r requirements.txt)

# Stop the session before patching system image
sudo -u "$REAL_USER" XDG_RUNTIME_DIR="/run/user/$(id -u "$REAL_USER")" waydroid session stop 2>/dev/null || true
sleep 2

(cd "$SCRIPT_DIR" && python3 main.py install libndk)

# 7. Permissions for the user to talk to waydroid
log "Adding $REAL_USER to required groups (waydroid, kvm)"
getent group waydroid >/dev/null && usermod -aG waydroid "$REAL_USER" || true
getent group kvm >/dev/null && usermod -aG kvm "$REAL_USER" || true

log ""
log "Setup complete. Next:"
log "  1. Log out and back in (for group membership)"
log "  2. Run: bash scripts/waydroid-run.sh"
log ""
log "If display server is X11 (not Wayland), you can still install/test the APK"
log "via adb without launching the UI."
