//go:build linux
// +build linux

// Package platform — device_info_linux.go provides identifiers sent to
// HWID-binding subscription providers (SPEC 061-F-N §"Request headers").
//
// Values cached at first call (see device_info_darwin.go for rationale).
// Reads pseudo-FS only — no exec, no privileges required.
package platform

import (
	"os"
	"strings"
	"sync"
)

var (
	deviceInfoOnce       sync.Once
	cachedDeviceOSVer    string
	cachedDeviceModelStr string
)

// DeviceOS returns the OS family string sent in `X-Device-OS`.
func DeviceOS() string { return "linux" }

// DeviceOSVersion returns the distro release identifier (e.g. `22.04`).
// Sourced from `/etc/os-release` `VERSION_ID=...`. Cached.
func DeviceOSVersion() string {
	deviceInfoOnce.Do(loadDeviceInfo)
	return cachedDeviceOSVer
}

// DeviceModel returns the chassis model string (e.g. `ThinkPad X1 Carbon`).
// Sourced from `/sys/devices/virtual/dmi/id/product_name`. Cached.
func DeviceModel() string {
	deviceInfoOnce.Do(loadDeviceInfo)
	return cachedDeviceModelStr
}

func loadDeviceInfo() {
	cachedDeviceOSVer = readOSReleaseValue("VERSION_ID")
	if cachedDeviceOSVer == "" {
		cachedDeviceOSVer = "unknown"
	}
	cachedDeviceModelStr = readFileTrim("/sys/devices/virtual/dmi/id/product_name")
	if cachedDeviceModelStr == "" {
		cachedDeviceModelStr = "unknown"
	}
}

// readOSReleaseValue parses /etc/os-release for `KEY=value`, handling
// optional surrounding quotes per the freedesktop os-release spec.
// Empty string on any error.
func readOSReleaseValue(key string) string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	prefix := key + "="
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		v := strings.TrimPrefix(line, prefix)
		v = strings.Trim(v, `"'`)
		return strings.TrimSpace(v)
	}
	return ""
}

// readFileTrim reads file at path, returning trimmed contents or "" on error.
func readFileTrim(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
