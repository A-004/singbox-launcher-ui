//go:build darwin
// +build darwin

// Package platform — device_info_darwin.go provides the per-machine
// identifiers sent to subscription providers (Remnawave / Marzneshin /
// Marzban-style HWID-binding panels) per SPEC 061-F-N §"Request headers".
//
// All values are cached at first call: querying `sw_vers` / `sysctl` per
// fetch would add ~100 ms each — irrelevant for one Update, painful when
// auto-refresh sweeps 50 subscriptions. Cache is package-scoped so it
// survives until the process exits.
//
// On any error → `unknown`. Provider will still see the X-Hwid + X-Device-OS
// (which never fail) — only the model/version are best-effort.
package platform

import (
	"bytes"
	"os/exec"
	"strings"
	"sync"
)

var (
	deviceInfoOnce       sync.Once
	cachedDeviceOSVer    string
	cachedDeviceModelStr string
)

// DeviceOS returns the OS family string sent in `X-Device-OS`.
// macOS / windows / linux — canonical case per Remnawave docs.
func DeviceOS() string { return "macOS" }

// DeviceOSVersion returns the OS release identifier (e.g. `15.2`).
// Sourced from `sw_vers -productVersion`. Cached.
func DeviceOSVersion() string {
	deviceInfoOnce.Do(loadDeviceInfo)
	return cachedDeviceOSVer
}

// DeviceModel returns the Apple hardware model string (e.g. `MacBookPro18,1`).
// Sourced from `sysctl -n hw.model`. Cached.
func DeviceModel() string {
	deviceInfoOnce.Do(loadDeviceInfo)
	return cachedDeviceModelStr
}

func loadDeviceInfo() {
	cachedDeviceOSVer = runTrim("sw_vers", "-productVersion")
	if cachedDeviceOSVer == "" {
		cachedDeviceOSVer = "unknown"
	}
	cachedDeviceModelStr = runTrim("sysctl", "-n", "hw.model")
	if cachedDeviceModelStr == "" {
		cachedDeviceModelStr = "unknown"
	}
}

// runTrim runs cmd and returns its stdout with surrounding whitespace
// stripped. Empty string on any error (caller substitutes `unknown`).
func runTrim(name string, args ...string) string {
	var stdout bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(stdout.String())
}
