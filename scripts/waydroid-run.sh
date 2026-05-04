#!/usr/bin/env bash
# Boot Waydroid, install the wt APK, launch it, tail logs.
# Run after: sudo bash scripts/waydroid-setup.sh
#
# Subcommands:
#   start    Start container + Weston + session.
#   install  Stage APK into container's /data and pm install.
#   run      Launch the app and tail filtered logcat.
#   logcat   Tail filtered logcat live.
#   sherpa   Run libsherpa-asr.so --help inside container as a translator smoke test.
#   stop     Stop session (container service stays up).
#   status   Show container/session state and app pid.
#   all      start + install + run (default).

set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

VERSION="$(awk -F'[\x27\x22]' '/^  VERSION:/{print $2; exit}' Taskfile.yml)"
: "${APK_PATH:=dist/wt-${VERSION}.apk}"
: "${PKG:=com.asolopovas.wtranscribe}"
: "${ACTIVITY:=org.golang.app.GoNativeActivity}"
: "${WAY_USER_DATA:=$HOME/.local/share/waydroid/data}"

XDG_DIR="/run/user/$(id -u)"
WSHELL=(sudo waydroid shell --)

log() { printf '\033[1;36m[waydroid-run]\033[0m %s\n' "$*" >&2; }
die() {
	printf '\033[1;31m[waydroid-run] ERROR:\033[0m %s\n' "$*" >&2
	exit 1
}

ensure_weston() {
	if [ -z "${WAYLAND_DISPLAY:-}" ] || [ ! -S "$XDG_DIR/${WAYLAND_DISPLAY}" ]; then
		if [ ! -S "$XDG_DIR/wayland-1" ]; then
			command -v weston >/dev/null || die "weston not installed: sudo apt install weston"
			log "Starting headless Weston compositor on wayland-1"
			setsid weston --backend=headless-backend.so --socket=wayland-1 \
				--width=1080 --height=2400 >/tmp/weston.log 2>&1 </dev/null &
			for _ in $(seq 1 20); do
				[ -S "$XDG_DIR/wayland-1" ] && break
				sleep 0.5
			done
			[ -S "$XDG_DIR/wayland-1" ] || die "weston failed to create wayland-1 socket — see /tmp/weston.log"
		fi
		export WAYLAND_DISPLAY=wayland-1
	fi
	log "WAYLAND_DISPLAY=$WAYLAND_DISPLAY"
}

ensure_container() {
	systemctl is-active --quiet waydroid-container.service || {
		log "Starting waydroid-container.service"
		sudo systemctl start waydroid-container.service
		sleep 3
	}
}

ensure_session() {
	if ! waydroid status 2>/dev/null | grep -q "Session:.*RUNNING"; then
		log "Starting Waydroid session"
		setsid waydroid session start >/tmp/waydroid-session.log 2>&1 </dev/null &
		for _ in $(seq 1 60); do
			waydroid status 2>/dev/null | grep -q "Session:.*RUNNING" && break
			sleep 2
		done
	fi
	# Wait for Android boot inside.
	for _ in $(seq 1 60); do
		bc=$("${WSHELL[@]}" /system/bin/getprop sys.boot_completed 2>/dev/null | tr -d '\r')
		[ "$bc" = "1" ] && return
		sleep 2
	done
	die "Android did not finish booting inside Waydroid"
}

cmd_start() {
	ensure_weston
	ensure_container
	ensure_session
	log "Container ready. ABI list: $("${WSHELL[@]}" /system/bin/getprop ro.product.cpu.abilist | tr -d '\r')"
	log "Native bridge:           $("${WSHELL[@]}" /system/bin/getprop ro.dalvik.vm.native.bridge | tr -d '\r')"
}

cmd_install() {
	[ -f "$APK_PATH" ] || die "APK not found: $APK_PATH"
	cmd_start
	log "Staging APK into container's /data/local/tmp"
	# WAY_USER_DATA is bind-mounted into the container as /data; we need sudo to write into it.
	sudo mkdir -p "$WAY_USER_DATA/local/tmp"
	sudo cp "$APK_PATH" "$WAY_USER_DATA/local/tmp/wt.apk"
	log "Running pm install"
	"${WSHELL[@]}" /system/bin/pm install -r -d /data/local/tmp/wt.apk
	for p in RECORD_AUDIO POST_NOTIFICATIONS; do
		"${WSHELL[@]}" /system/bin/pm grant "$PKG" "android.permission.$p" 2>/dev/null || true
	done
	log "Installed."
}

cmd_run() {
	cmd_start
	"${WSHELL[@]}" /system/bin/logcat -c 2>/dev/null || true
	log "Launching $PKG/$ACTIVITY"
	"${WSHELL[@]}" /system/bin/am start -n "$PKG/$ACTIVITY"
	sleep 5
	local pid
	pid=$("${WSHELL[@]}" /system/bin/pidof "$PKG" 2>/dev/null | tr -d '\r')
	if [ -n "$pid" ]; then
		log "App alive: PID=$pid"
		"${WSHELL[@]}" /system/bin/sh -c "cat /proc/$pid/status 2>/dev/null | grep -E 'VmRSS|State'" | sed 's/^/  /'
	else
		log "App not running — check logcat"
	fi
	cmd_logcat 10
}

cmd_logcat() {
	local seconds="${1:-30}"
	log "Tailing filtered logcat for ${seconds}s..."
	timeout "$seconds" sudo waydroid shell -- /system/bin/logcat -v brief \
		wtranscribe:V GoLog:V Go:V wt:V WtNative:V \
		AndroidRuntime:E DEBUG:E libc:F nativebridge:V "*:S" 2>&1 || true
}

cmd_sherpa() {
	cmd_start
	log "Running libsherpa-asr.so as ELF (translator smoke test)"
	"${WSHELL[@]}" /system/bin/sh -c "
		LIBDIR=\$(echo /data/app/*/com.asolopovas.wtranscribe*/lib/arm64 2>/dev/null);
		[ -z \"\$LIBDIR\" ] && { echo 'APK not installed yet'; exit 1; };
		cp \$LIBDIR/libsherpa-asr.so /data/local/tmp/sherpa-asr;
		chmod +x /data/local/tmp/sherpa-asr;
		/data/local/tmp/sherpa-asr --help 2>&1 | head -20
	"
}

cmd_status() {
	waydroid status 2>&1 | sed 's/^/  /' || true
	echo
	local pid
	pid=$("${WSHELL[@]}" /system/bin/pidof "$PKG" 2>/dev/null | tr -d '\r')
	if [ -n "$pid" ]; then
		echo "  App PID:      $pid"
		"${WSHELL[@]}" /system/bin/sh -c "cat /proc/$pid/status 2>/dev/null | grep -E 'VmRSS|State'" | sed 's/^/  /'
	else
		echo "  App:          not running"
	fi
}

cmd_stop() {
	log "Stopping Waydroid session (container service stays up)"
	waydroid session stop 2>/dev/null || true
}

cmd_all() {
	cmd_start
	cmd_install
	cmd_run
}

case "${1:-all}" in
start | install | run | logcat | sherpa | status | stop | all) "cmd_${1:-all}" ;;
-h | --help | help) sed -n '2,15p' "$0" ;;
*) die "unknown subcommand: $1" ;;
esac
