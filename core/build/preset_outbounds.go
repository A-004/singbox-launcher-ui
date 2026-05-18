// Package build — see preset_outbounds.go for SPEC 056 pre-patch core.
//
// File preset_outbounds.go (SPEC 056) — реализация preset.outbounds через
// **pre-patch** parserCfg.ParserConfig.Outbounds[] **до** запуска native
// outbound generator'а.
//
// Архитектурный принцип: preset.outbounds — это parser-format (зеркалит
// configtypes.OutboundConfig), а не post-merge JSON-патч. ExpandPresetOutbounds
// конвертит template.PresetOutbound[i] → configtypes.OutboundConfig (с
// vars-substitution + if/if_or filter), затем ApplyPresetOutboundsToParserConfig
// применяет add/update к **deep-clone** parserCfg.ParserConfig.Outbounds[] в
// порядке state.RulesV6.
//
// Дальше нативный 3-pass GenerateOutboundsFromParserConfig сам делает всё,
// что нужно sing-box'у: options-flatten, filters→filterNodesForSelector,
// addOutbounds union, comment-prefix "// %s\n". Ни одной strip/sanitize/
// transform функции для outbound JSON — это и есть основной выигрыш по
// сравнению с предыдущей реализацией SPEC 055 (post-merge → каскад strip'ов).
//
// См. SPECS/056-B-N-OUTBOUNDS_PARSER_RESTORE/SPEC.md.
package build

import (
	"bytes"
	"encoding/json"
	"fmt"

	"singbox-launcher/core/config/configtypes"
	v6 "singbox-launcher/core/state/v6"
	"singbox-launcher/core/template"
)

// presetOutboundEntry — internal: разделяет режим применения и сам
// configtypes.OutboundConfig (без control-полей mode/if/if_or).
//
// Возвращается ExpandPresetOutbounds и потребляется в
// ApplyPresetOutboundsToParserConfig. PresetID нужен только для warning'ов.
type presetOutboundEntry struct {
	Mode     string // "add" | "update"
	Config   configtypes.OutboundConfig
	PresetID string
}

// ApplyPresetOutboundsToParserConfig возвращает копию parserCfg с применёнными
// preset.outbounds[] от всех enabled preset-ref'ов из rules.
//
// Mutates: ничего — оригинал parserCfg deep-cloned перед изменениями
// (acceptance #7 SPEC 056: disable preset → effect полностью исчезает,
// потому что original parser_config never touched).
//
// Алгоритм:
//
//  1. Deep-clone parserCfg (JSON round-trip).
//  2. Собрать map[tag]→index по cloned.ParserConfig.Outbounds[] и "global tags"
//     (для дифференциации warning'ов на collision: с global vs earlier preset).
//  3. Walk rules в их порядке (Kind=preset && Enabled), для каждого:
//     a. lookup preset by Ref;
//     b. decode body.vars;
//     c. ExpandPresetOutbounds(preset, vars) → []presetOutboundEntry;
//     d. для каждой entry:
//     - mode="add":
//     • tag в map → identical body → silent skip (warning класса DEBUG),
//     иначе first wins + warning (с указанием "global" vs "earlier preset")
//     • новый tag → append + update map
//     - mode="update":
//     • tag не в map → warning, no-op (no auto-create)
//     • tag в map → applyOutboundUpdate(target, entry.Config) → replace
//  4. Возвращает (cloned, warnings, nil).
//
// Errors: возвращает (nil, nil, err) только если parserCfg сам nil.
// JSON-marshal/unmarshal на clone'е — internal-only path с фиксированной
// структурой; ошибка тут = bug в Go runtime, не user-facing.
func ApplyPresetOutboundsToParserConfig(
	parserCfg *configtypes.ParserConfig,
	presets []template.Preset,
	rules []v6.Rule,
) (*configtypes.ParserConfig, []string, error) {
	if parserCfg == nil {
		return nil, nil, fmt.Errorf("ApplyPresetOutboundsToParserConfig: nil parserCfg")
	}

	cloned, err := cloneParserConfig(parserCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("ApplyPresetOutboundsToParserConfig: clone parserCfg: %w", err)
	}

	// Quick out — no enabled rules → return clone (acceptance: idempotent
	// rebuild без активных preset'ов даёт parser_config-byte-for-byte equal
	// исходному; разница только в указателе).
	if !hasAnyV6Rule(rules) {
		return cloned, nil, nil
	}

	presetByID := make(map[string]*template.Preset, len(presets))
	for i := range presets {
		presetByID[presets[i].ID] = &presets[i]
	}

	outbounds := cloned.ParserConfig.Outbounds
	tagToIndex := make(map[string]int, len(outbounds))
	for i, o := range outbounds {
		tagToIndex[o.Tag] = i
	}
	// globalTags фиксируется ДО первого apply — используется чтобы в
	// warning'е на collision сказать "с template global" vs "с earlier
	// preset". (При втором preset.add того же tag'а tagToIndex уже содержит
	// preset-added entry от первого preset'а — и это для warning'а другой
	// семантический класс.)
	globalTags := make(map[string]bool, len(outbounds))
	for _, o := range outbounds {
		globalTags[o.Tag] = true
	}

	var warnings []string

	for _, rule := range rules {
		if !rule.Enabled || rule.Kind != v6.RuleKindPreset {
			continue
		}
		preset, ok := presetByID[rule.Ref]
		if !ok {
			continue // unknown preset ref — preset_merge уже warning'ит
		}
		body, err := rule.DecodeBody()
		if err != nil {
			warnings = append(warnings, fmt.Sprintf(
				"preset outbounds: decode body for ref %q: %v", rule.Ref, err))
			continue
		}
		pb, _ := body.(*v6.PresetBody)
		if pb == nil {
			continue
		}
		entries, expandWarns := ExpandPresetOutbounds(preset, pb.Vars)
		for _, w := range expandWarns {
			warnings = append(warnings, "preset outbounds: "+w.String())
		}

		for _, entry := range entries {
			tag := entry.Config.Tag
			switch entry.Mode {
			case "add":
				if idx, has := tagToIndex[tag]; has {
					existing := outbounds[idx]
					if outboundsIdentical(existing, entry.Config) {
						// Identical body — silent skip (бессмысленно дублировать;
						// возникает естественно когда ru-inside+russian оба добавляют
						// "ru VPN 🇷🇺" с тем же body — copy-paste, не баг).
						continue
					}
					src := "earlier preset"
					if globalTags[tag] {
						src = "template global"
					}
					warnings = append(warnings, fmt.Sprintf(
						"preset %q: outbound add tag %q collides with %s "+
							"(first wins; this entry skipped)",
						entry.PresetID, tag, src))
					continue
				}
				outbounds = append(outbounds, entry.Config)
				tagToIndex[tag] = len(outbounds) - 1

			case "update":
				idx, has := tagToIndex[tag]
				if !has {
					warnings = append(warnings, fmt.Sprintf(
						"preset %q: outbound update target tag %q not found "+
							"(no auto-create; skipped)",
						entry.PresetID, tag))
					continue
				}
				outbounds[idx] = applyOutboundUpdate(outbounds[idx], entry.Config)
			}
		}
	}

	cloned.ParserConfig.Outbounds = outbounds
	return cloned, warnings, nil
}

// ExpandPresetOutbounds разворачивает preset.Outbounds[] в []presetOutboundEntry
// с уже применённой substitution @var и if/if_or фильтрацией.
//
// userVars — значения переменных из state.rule.body.vars (только diff от
// template default'ов; пустые/отсутствующие резолвятся через
// preset.vars[].Default).
//
// Алгоритм идентичен ExpandPreset (rule/rule_set/dns_rule path), но для
// каждой entry дополнительно:
//   - normalizes Mode ("" → "add"; loader уже зачистил unknown);
//   - JSON round-trip через map для substitute @var;
//   - drop control-полей (mode/if/if_or) из map ДО unmarshal в OutboundConfig;
//   - типизированный re-unmarshal в configtypes.OutboundConfig.
//
// На unresolved @var — entry skip + warning, остальные entries продолжают
// обрабатываться (в отличие от ExpandPreset который отменяет весь preset
// на unresolved — там dangling @var в rule_set/rule может всё разломать,
// здесь же одна сломанная entry не блокирует другие).
func ExpandPresetOutbounds(preset *template.Preset, userVars map[string]string) ([]presetOutboundEntry, []ExpandWarning) {
	if preset == nil || len(preset.Outbounds) == 0 {
		return nil, nil
	}

	// === 1. Build varsMap (тот же паттерн что в ExpandPreset). ===
	varsMap := make(map[string]string, len(preset.Vars))
	for _, v := range preset.Vars {
		if userVal, ok := userVars[v.Name]; ok && userVal != "" {
			varsMap[v.Name] = userVal
		} else {
			varsMap[v.Name] = v.Default
		}
	}

	// === 2. Filter vars по if/if_or; неактивные → удалить из map. ===
	activeVars := filterActiveVars(preset.Vars, varsMap)
	for name := range varsMap {
		if !activeVars[name] {
			delete(varsMap, name)
		}
	}

	var warnings []ExpandWarning
	out := make([]presetOutboundEntry, 0, len(preset.Outbounds))

	for i := range preset.Outbounds {
		ob := preset.Outbounds[i]

		// === 3. Filter by entry if/if_or. ===
		if !evalIf(ob.If, ob.IfOr, varsMap) {
			continue
		}

		mode := ob.Mode
		if mode == "" {
			mode = "add"
		}

		// === 4. Marshal → map → substitute → strip control → unmarshal. ===
		raw, err := json.Marshal(ob)
		if err != nil {
			warnings = append(warnings, ExpandWarning{
				PresetID: preset.ID,
				Message:  fmt.Sprintf("outbounds[%d] (tag=%q): marshal: %v", i, ob.Tag, err),
			})
			continue
		}
		var asMap map[string]interface{}
		if err := json.Unmarshal(raw, &asMap); err != nil {
			warnings = append(warnings, ExpandWarning{
				PresetID: preset.ID,
				Message:  fmt.Sprintf("outbounds[%d] (tag=%q): unmarshal: %v", i, ob.Tag, err),
			})
			continue
		}
		substituted, ok := substituteAny(asMap, varsMap)
		if !ok {
			warnings = append(warnings, ExpandWarning{
				PresetID: preset.ID,
				Message: fmt.Sprintf(
					"outbounds[%d] (tag=%q): unresolved @var (entry skipped)",
					i, ob.Tag),
			})
			continue
		}
		substMap, _ := substituted.(map[string]interface{})
		if substMap == nil {
			continue
		}
		// Control-поля не должны попасть в configtypes.OutboundConfig
		// (он их и не имеет — strict-decoder бы ругался; но native генератор
		// потом маршалит OutboundConfig обратно через json.Marshal, где
		// неизвестные ключи не возникают, так что strip — defensive).
		delete(substMap, "mode")
		delete(substMap, "if")
		delete(substMap, "if_or")

		finalRaw, err := json.Marshal(substMap)
		if err != nil {
			warnings = append(warnings, ExpandWarning{
				PresetID: preset.ID,
				Message: fmt.Sprintf(
					"outbounds[%d] (tag=%q): re-marshal: %v", i, ob.Tag, err),
			})
			continue
		}
		var oc configtypes.OutboundConfig
		if err := json.Unmarshal(finalRaw, &oc); err != nil {
			warnings = append(warnings, ExpandWarning{
				PresetID: preset.ID,
				Message: fmt.Sprintf(
					"outbounds[%d] (tag=%q): re-unmarshal to OutboundConfig: %v",
					i, ob.Tag, err),
			})
			continue
		}
		out = append(out, presetOutboundEntry{
			Mode:     mode,
			Config:   oc,
			PresetID: preset.ID,
		})
	}
	return out, warnings
}

// applyOutboundUpdate — типизированный field-merge patch'а в target.
//
// Возвращает НОВУЮ структуру (target value-immutable; AddOutbounds/Options/
// Filters/PreferredDefault внутри — fresh-allocated через cloneOptions/union).
//
// Семантика полей (см. PresetOutbound docstring):
//   - Type, Tag    — НЕ меняются (immutable; Type loader уже зачищает для update)
//   - Filters      — replace целиком (если patch.Filters != nil)
//   - AddOutbounds — union (preserve order, dedupe)
//   - Options.*    — per-key replace в target.Options (нет глубокого merge)
//   - PreferredDefault — replace
//   - Wizard       — replace
//   - Comment      — replace iff patch.Comment != ""
func applyOutboundUpdate(target, patch configtypes.OutboundConfig) configtypes.OutboundConfig {
	out := target

	if patch.Filters != nil {
		out.Filters = cloneOptions(patch.Filters)
	}
	if len(patch.AddOutbounds) > 0 {
		out.AddOutbounds = unionStringList(target.AddOutbounds, patch.AddOutbounds)
	}
	if len(patch.Options) > 0 {
		merged := cloneOptions(target.Options)
		if merged == nil {
			merged = make(map[string]interface{}, len(patch.Options))
		}
		for k, v := range patch.Options {
			merged[k] = v
		}
		out.Options = merged
	}
	if patch.PreferredDefault != nil {
		out.PreferredDefault = cloneOptions(patch.PreferredDefault)
	}
	if patch.Wizard != nil {
		out.Wizard = patch.Wizard
	}
	if patch.Comment != "" {
		out.Comment = patch.Comment
	}
	return out
}

// cloneParserConfig — deep-copy *configtypes.ParserConfig через JSON round-trip.
//
// Используется для immutable-input гарантии: оригинал parserCfg (из template
// или state) НЕ должен меняться при apply preset.outbounds. JSON-марш на
// маленькой структуре (~10-30 outbounds) — микросекунды, не bottleneck.
func cloneParserConfig(in *configtypes.ParserConfig) (*configtypes.ParserConfig, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var out configtypes.ParserConfig
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// outboundsIdentical — true если две configtypes.OutboundConfig дают
// **точно** одинаковый JSON. Используется для silent-skip на collision'е
// одинаковых mode=add entries (ru-inside и russian оба добавляют "ru VPN 🇷🇺"
// с identical body — copy-paste, не warning).
//
// Метод "byte-equal JSON" работает потому что:
//   - encoding/json эмитит map[string]interface{} ключи в lexicographic
//     order (deterministic);
//   - struct поля идут в declaration order (deterministic);
//   - omitempty для нулевых значений (deterministic).
//
// Edge case: разные nil vs empty slice/map (Filters nil vs Filters{}) — JSON
// видит их одинаково благодаря omitempty, так что safe.
func outboundsIdentical(a, b configtypes.OutboundConfig) bool {
	raw1, err1 := json.Marshal(a)
	raw2, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return bytes.Equal(raw1, raw2)
}

// cloneOptions — deep-copy map[string]interface{} через JSON round-trip.
// nil-вход → nil-выход (без аллокации пустого map'а — caller проверяет).
func cloneOptions(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

// unionStringList — union двух []string preserving первое вхождение
// (a-первый, b-второй); dedup case-sensitive.
//
// Используется для merge'а AddOutbounds в mode=update: preset.AddOutbounds
// добавляются ПОСЛЕ target.AddOutbounds (template-defined идут первыми,
// preset-added — после). Это сохраняет стабильность ordering'а в UI selector'е.
func unionStringList(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
