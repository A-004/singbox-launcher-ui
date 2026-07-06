package subscription

import (
	"strconv"
	"strings"

	"singbox-launcher/core/config/configtypes"
	"singbox-launcher/internal/debuglog"
)

// isValidHysteria2ObfsType checks if the obfs type is supported by sing-box for Hysteria2
// According to sing-box documentation, only "salamander" is supported
func isValidHysteria2ObfsType(obfsType string) bool {
	return obfsType == "salamander"
}

// buildHysteria2Outbound builds outbound configuration for Hysteria2 protocol.
//
// SPEC 063: always emits the full set of sing-box hysteria2 fields with
// factory defaults, so the per-node JSON editor shows every tunable field
// even when the subscription URI does not carry them. Only fields that are
// sing-box native to type=hysteria2 are included — xray-transport wrappers
// (congestion / quicParams / hysteriaSettings) are omitted.
func buildHysteria2Outbound(node *configtypes.ParsedNode, outbound map[string]interface{}) {
	// Password is required (stored in UUID field from userinfo)
	if node.UUID != "" {
		outbound["password"] = node.UUID
	} else {
		debuglog.WarnLog("Parser: Hysteria2 link missing password. URI might be invalid.")
	}

	// Mux — always present with factory defaults so the editor shows it.
	outbound["mux"] = map[string]interface{}{
		"enabled":         false,
		"concurrency":     -1,
		"xudpConcurrency": 8,
		"xudpProxyUDP443": "",
	}

	// Optional: mport / ports query — Hysteria2 multi-port.
	mport := strings.TrimSpace(queryGetFold(node.Query, "mport"))
	if mport == "" {
		mport = strings.TrimSpace(queryGetFold(node.Query, "ports"))
	}
	if sp := hysteria2MportSpecToSingBoxServerPorts(mport); len(sp) > 0 {
		outbound["server_ports"] = sp
	}

	// Optional: bandwidth (up/down in Mbps) — from URI, else 0 for editor.
	upSet := false
	if up := node.Query.Get("upmbps"); up != "" {
		if upMBps, err := strconv.Atoi(up); err == nil {
			outbound["up_mbps"] = upMBps
			upSet = true
		}
	}
	if !upSet {
		outbound["up_mbps"] = 0
	}
	downSet := false
	if down := node.Query.Get("downmbps"); down != "" {
		if downMBps, err := strconv.Atoi(down); err == nil {
			outbound["down_mbps"] = downMBps
			downSet = true
		}
	}
	if !downSet {
		outbound["down_mbps"] = 0
	}

	// Optional: obfs (obfuscation) — from URI, else omit (not present → no obfs).
	if obfs := node.Query.Get("obfs"); obfs != "" {
		if !isValidHysteria2ObfsType(obfs) {
			debuglog.WarnLog("Parser: Invalid or unsupported Hysteria2 obfs type '%s'. Only 'salamander' is supported. Skipping obfs.", obfs)
		} else {
			obfsConfig := map[string]interface{}{
				"type": obfs,
			}
			if obfsPassword := node.Query.Get("obfs-password"); obfsPassword != "" {
				obfsConfig["password"] = obfsPassword
			}
			outbound["obfs"] = obfsConfig
		}
	}

	// TLS settings (required for hysteria2)
	buildHysteria2TLS(node, outbound)
}

// buildHysteria2TLS builds the full TLS block for a Hysteria2 outbound.
//
// SPEC 063: always emits every known TLS field with factory defaults so the
// per-node editor shows the complete structure.
func buildHysteria2TLS(node *configtypes.ParsedNode, outbound map[string]interface{}) {
	q := node.Query
	sni := queryGetFold(q, "sni")

	tlsData := map[string]interface{}{
		"enabled": true,
	}

	// server_name
	if sni != "" && sni != "🔒" && (strings.Contains(sni, ".") || strings.Contains(sni, ":")) {
		tlsData["server_name"] = sni
	} else if node.Server != "" {
		tlsData["server_name"] = node.Server
	}

	// insecure — always emit so editor shows the toggle.
	tlsData["insecure"] = tlsInsecureTrue(q) ||
		queryGetFold(q, "skip-cert-verify") == "true" ||
		queryGetFold(q, "skip-cert-verify") == "1"

	// ALPN — from URI, else default to ["h3"] for hy2.
	if alpn := queryGetFold(q, "alpn"); alpn != "" {
		alpnList := strings.Split(alpn, ",")
		for i := range alpnList {
			alpnList[i] = strings.TrimSpace(alpnList[i])
		}
		tlsData["alpn"] = alpnList
	} else {
		tlsData["alpn"] = []string{"h3"}
	}

	// NOTE: uTLS fingerprint is NOT applicable to Hysteria2.
	// Hysteria2 uses QUIC (UDP), and uTLS (TLS fingerprint imitation) works only
	// over TCP+TLS. Adding utls block to hysteria2 causes TLS handshake to hang.
	// Any fp/fingerprint URI params are silently ignored.

	// certificate_public_key_sha256 (pinSHA256)
	if pin := strings.TrimSpace(queryGetFold(q, "pinSHA256")); pin != "" {
		tlsData["certificate_public_key_sha256"] = []string{pin}
	}

	outbound["tls"] = tlsData
}
