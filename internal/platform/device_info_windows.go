//go:build windows
// +build windows

// Package platform — device_info_windows.go provides identifiers sent to
// HWID-binding subscription providers (SPEC 061-F-N §"Request headers").
//
// Values cached at first call (see device_info_darwin.go for rationale).
// `wmic` is deprecated in modern Windows but still present in Win10+ default
// install — sufficient for our user base. Fallback `unknown` on any error.
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
func DeviceOS() string { return "windows" }

// DeviceOSVersion returns the Windows build version (e.g. `10.0.19045`).
// Sourced from `wmic os get Version /value`. Cached.
func DeviceOSVersion() string {
	deviceInfoOnce.Do(loadDeviceInfo)
	return cachedDeviceOSVer
}

// DeviceModel returns the system model string (e.g. `Surface Pro 9`).
// Sourced from `wmic computersystem get Model /value`. Cached.
func DeviceModel() string {
	deviceInfoOnce.Do(loadDeviceInfo)
	return cachedDeviceModelStr
}

func loadDeviceInfo() {
	cachedDeviceOSVer = wmicValue("os", "Version")
	if cachedDeviceOSVer == "" {
		cachedDeviceOSVer = "unknown"
	}
	cachedDeviceModelStr = wmicValue("computersystem", "Model")
	if cachedDeviceModelStr == "" {
		cachedDeviceModelStr = "unknown"
	}
}

// wmicValue runs `wmic <alias> get <property> /value` and parses the
// `Property=Value` line. Empty string on any error (caller substitutes
// `unknown`).
func wmicValue(alias, property string) string {
	var stdout bytes.Buffer
	cmd := exec.Command("wmic", alias, "get", property, "/value")
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return ""
	}
	prefix := property + "="
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimRight(line, "\r\t ")
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}
