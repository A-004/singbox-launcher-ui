package v6

import (
	v5 "singbox-launcher/core/state/v5"
)

// State — корневая модель v6 (SPEC 053).
//
// Изменения vs v5:
//   - meta.version: 5 → 6
//   - meta.schema: новое поле "presets_v1"
//   - custom_rules[] → rules[] с kind discriminator (preset/inline/srs)
//   - config_params[] удалено (vars per-preset в body.vars)
//   - dns_options → dns с template_servers + extra_servers + extra_rules
//
// Без изменений (re-export из v5):
//   - connections (sources, outbounds, defaults)
//   - vars[] (глобальные template vars: cert_store, tun, route_final, ...)
type State struct {
	Meta        MetaSection           `json:"meta"`
	Connections v5.ConnectionsSection `json:"connections"`
	Rules       []Rule                `json:"rules"`
	Vars        []v5.SettingVar       `json:"vars,omitempty"`
	DNS         DNSConfig             `json:"dns"`
}

// MetaSection — мета v6. Добавлено поле Schema для будущего versioning'а.
type MetaSection struct {
	Version   int    `json:"version"`
	Schema    string `json:"schema,omitempty"`
	Comment   string `json:"comment,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
