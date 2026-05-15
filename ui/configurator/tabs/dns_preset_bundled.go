// File dns_preset_bundled.go — read-only секция DNS tab для bundled DNS-серверов
// от активных preset-ref правил.
//
// Юзер видит **что preset реально добавит** в config.json::dns.servers[] — это
// важно для понимания: при активации Russian domains preset с use_dns_override
// у тебя в DNS будут yandex_udp/doh/dot (один из них реально попадает в config —
// тот что выбран через @dns_server var).
//
// Меняется через preset edit dialog (var dns_server picker), не через DNS tab.
package tabs

import (
	"encoding/json"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	ttwidget "github.com/dweymouth/fyne-tooltip/widget"

	"singbox-launcher/core/build"
	wizardtemplate "singbox-launcher/core/template"
	wizardmodels "singbox-launcher/ui/configurator/models"
)

// lockLeading — visual placeholder под местом checkbox'а в DNS row.
// Возвращает 🔒 emoji с padding-spacer'ом справа, выровненный по ширине
// под checkbox. Используется вместо галочки для read-only bundled rows —
// одна позиция в left-edge как у обычных DNS-серверов с галочкой.
func lockLeading() fyne.CanvasObject {
	lockLabel := widget.NewLabel("🔒")
	// Spacer вдогонку чтобы emoji не прилипал к контенту справа —
	// match width стандартного CheckLeading (~28-32px).
	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(4, 0))
	return container.NewHBox(lockLabel, spacer)
}

// jsonMarshalIndent — thin alias for jsonPrettyMarshal callers (encapsulates std import).
func jsonMarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	return json.MarshalIndent(v, prefix, indent)
}

// renderPresetBundledDNSRows — собирает read-only DNS server rows для всех
// активных preset-ref'ов. Возвращает список widget'ов готовых к Add в VBox.
//
// parentWindow — для view dialog'а (показывает read-only детали bundled DNS).
//
// Только те bundled DNS-серверы которые ФАКТИЧЕСКИ попадают в config (то есть
// прошли через `filterDnsServers` в ExpandPreset — выбранные через @dns_server
// или указанные литералом в dns_rule.server). Преcет с use_dns_override=false
// не покажет никаких rows.
func renderPresetBundledDNSRows(m *wizardmodels.WizardModel, parentWindow fyne.Window) []fyne.CanvasObject {
	if m == nil || m.TemplateData == nil {
		return nil
	}
	presetByID := make(map[string]*wizardtemplate.Preset, len(m.TemplateData.Presets))
	for i := range m.TemplateData.Presets {
		presetByID[m.TemplateData.Presets[i].ID] = &m.TemplateData.Presets[i]
	}

	var rows []fyne.CanvasObject
	for _, pr := range m.PresetRefs {
		if pr == nil || !pr.Enabled {
			continue
		}
		tpl := presetByID[pr.Ref]
		if tpl == nil {
			continue
		}
		frags, _, ok := build.ExpandPreset(tpl, pr.Vars)
		if !ok || len(frags.DNSServers) == 0 {
			continue
		}
		for _, ds := range frags.DNSServers {
			dsCopy := ds
			tplCopy := tpl
			onView := func() { showBundledDNSDetailsDialog(parentWindow, tplCopy, dsCopy) }
			row := buildPresetBundledDNSRow(tplCopy, dsCopy, onView)
			if row != nil {
				rows = append(rows, row)
			}
		}
	}
	return rows
}

// renderPresetBundledDNSRulesRows — собирает read-only DNS rule rows для всех
// активных preset-ref'ов которые имеют preset.dns_rule.
func renderPresetBundledDNSRulesRows(m *wizardmodels.WizardModel, parentWindow fyne.Window) []fyne.CanvasObject {
	if m == nil || m.TemplateData == nil {
		return nil
	}
	presetByID := make(map[string]*wizardtemplate.Preset, len(m.TemplateData.Presets))
	for i := range m.TemplateData.Presets {
		presetByID[m.TemplateData.Presets[i].ID] = &m.TemplateData.Presets[i]
	}

	var rows []fyne.CanvasObject
	for _, pr := range m.PresetRefs {
		if pr == nil || !pr.Enabled {
			continue
		}
		tpl := presetByID[pr.Ref]
		if tpl == nil {
			continue
		}
		frags, _, ok := build.ExpandPreset(tpl, pr.Vars)
		if !ok || frags.DNSRule == nil {
			continue
		}
		ruleCopy := frags.DNSRule
		tplCopy := tpl
		onView := func() { showBundledDNSRuleDetailsDialog(parentWindow, tplCopy, ruleCopy) }
		rows = append(rows, buildPresetBundledDNSRuleRow(tplCopy, ruleCopy, onView))
	}
	return rows
}

// buildPresetBundledDNSRuleRow — read-only одна строка для bundled DNS-rule preset'а.
//
// Layout: `🔒  <preset-label> DNS rule (<rule_set_tags>)         [View]`
// Tooltip: `server=<server> · rule_set=<full-list>`
func buildPresetBundledDNSRuleRow(tpl *wizardtemplate.Preset, rule map[string]interface{}, onView func()) fyne.CanvasObject {
	// Соберём rule_set summary (local tag'и без preset-prefix).
	ruleSetSummary := ""
	switch v := rule["rule_set"].(type) {
	case string:
		ruleSetSummary = stripPresetPrefix(v, tpl.ID)
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, x := range v {
			if s, ok := x.(string); ok {
				parts = append(parts, stripPresetPrefix(s, tpl.ID))
			}
		}
		ruleSetSummary = joinSep(parts, ", ")
	}

	presetLabel := tpl.Label
	if presetLabel == "" {
		presetLabel = tpl.ID
	}

	rowText := presetLabel + " — DNS rule"
	if ruleSetSummary != "" {
		rowText += " (" + ruleSetSummary + ")"
	}
	titleLabel := ttwidget.NewLabel(rowText)
	titleLabel.Wrapping = fyne.TextTruncate

	// Tooltip: server + полный rule_set list.
	server, _ := rule["server"].(string)
	server = stripPresetPrefix(server, tpl.ID)
	tipParts := []string{}
	if server != "" {
		tipParts = append(tipParts, "server="+server)
	}
	if ruleSetSummary != "" {
		tipParts = append(tipParts, "rule_set="+ruleSetSummary)
	}
	if tip := joinSep(tipParts, " · "); tip != "" {
		titleLabel.SetToolTip(tip)
	}

	var rightCluster *fyne.Container
	if onView != nil {
		viewBtn := widget.NewButton("View JSON", onView)
		viewBtn.Importance = widget.LowImportance
		rightCluster = container.NewHBox(viewBtn)
	} else {
		rightCluster = container.NewHBox()
	}

	return container.NewBorder(nil, nil, lockLeading(), rightCluster, titleLabel)
}

// showBundledDNSDetailsDialog — read-only modal с full JSON bundled DNS server.
func showBundledDNSDetailsDialog(parent fyne.Window, tpl *wizardtemplate.Preset, ds map[string]interface{}) {
	body, _ := jsonPrettyMarshal(ds)
	showBundledReadOnlyDetails(parent, tpl, "DNS server details", body)
}

// showBundledDNSRuleDetailsDialog — read-only modal с DNS-rule preset'а.
// Изменения preset-bundled DNS rule НЕВОЗМОЖНЫ — содержимое определяется template'ом
// + значениями vars. Юзер для кастомных DNS-rules использует Extra rules editor
// (внизу DNS tab) — это полностью отдельный механизм.
func showBundledDNSRuleDetailsDialog(parent fyne.Window, tpl *wizardtemplate.Preset, rule map[string]interface{}) {
	body, _ := jsonPrettyMarshal(rule)
	showBundledReadOnlyDetails(parent, tpl, "DNS rule details", body)
}

// showBundledReadOnlyDetails — модал с monospace JSON preview через RichText,
// без редактирования (read-only). Юзер может выделять/копировать текст.
func showBundledReadOnlyDetails(parent fyne.Window, tpl *wizardtemplate.Preset, title, jsonBody string) {
	if parent == nil {
		return
	}
	desc := tpl.Label
	if desc == "" {
		desc = tpl.ID
	}
	header := widget.NewLabelWithStyle(
		"🔒  From preset: "+desc,
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true},
	)
	helpLabel := widget.NewLabelWithStyle(
		"Read-only. Edit via preset variables (Rules tab → Edit). For custom DNS rules use the Extra rules editor below.",
		fyne.TextAlignLeading, fyne.TextStyle{Italic: true},
	)
	helpLabel.Wrapping = fyne.TextWrapWord

	jsonRich := widget.NewRichTextFromMarkdown("```json\n" + jsonBody + "\n```")
	jsonRich.Wrapping = fyne.TextWrapWord
	scroll := container.NewScroll(jsonRich)

	content := container.NewBorder(
		container.NewVBox(header, helpLabel),
		nil, nil, nil,
		scroll,
	)
	d := dialog.NewCustom(title, "Close", content, parent)
	d.Resize(fyne.NewSize(560, 440))
	d.Show()
}

// jsonPrettyMarshal — JSON pretty-print для bundled DNS server / rule deтail dialog'а.
func jsonPrettyMarshal(v interface{}) (string, error) {
	b, err := jsonMarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// buildPresetBundledDNSRow — read-only одна строка для bundled DNS-сервера.
//
// Layout: `🔒  <preset-label> (<tag>)                        [View]`
// Tooltip: `type · server[:port] · description` (детали скрыты до hover).
func buildPresetBundledDNSRow(tpl *wizardtemplate.Preset, ds map[string]interface{}, onView func()) fyne.CanvasObject {
	tag, _ := ds["tag"].(string)
	typ, _ := ds["type"].(string)
	server, _ := ds["server"].(string)
	desc, _ := ds["description"].(string)

	localTag := stripPresetPrefix(tag, tpl.ID)
	presetLabel := tpl.Label
	if presetLabel == "" {
		presetLabel = tpl.ID
	}

	// Single-line title: "<preset-label> (<tag>)". 🔒 идёт **слева** на месте
	// чекбокса (через lockLeading), не в самом тексте — visual consistency
	// с обычными DNS-server rows которые имеют checkbox в той же позиции.
	rowText := presetLabel
	if localTag != "" {
		rowText += " (" + localTag + ")"
	}
	titleLabel := ttwidget.NewLabel(rowText)
	titleLabel.Wrapping = fyne.TextTruncate

	// Tooltip: details (type, server, description) — hover to see.
	tipParts := []string{}
	if typ != "" {
		tipParts = append(tipParts, typ)
	}
	if server != "" {
		tipParts = append(tipParts, server)
	}
	if desc != "" {
		tipParts = append(tipParts, desc)
	}
	if tip := joinSep(tipParts, " · "); tip != "" {
		titleLabel.SetToolTip(tip)
	}

	var rightCluster *fyne.Container
	if onView != nil {
		viewBtn := widget.NewButton("View JSON", onView)
		viewBtn.Importance = widget.LowImportance
		rightCluster = container.NewHBox(viewBtn)
	} else {
		rightCluster = container.NewHBox()
	}

	// Border: left = 🔒 (вместо чекбокса), center = title, right = View.
	return container.NewBorder(nil, nil, lockLeading(), rightCluster, titleLabel)
}

// stripPresetPrefix — убирает `<preset_id>:` префикс из tag'а если он там есть.
// `"ru-direct:yandex_udp"` → `"yandex_udp"`. Если префикса нет — возвращает as is.
func stripPresetPrefix(tag, presetID string) string {
	prefix := presetID + ":"
	if presetID != "" && len(tag) > len(prefix) && tag[:len(prefix)] == prefix {
		return tag[len(prefix):]
	}
	return tag
}

// joinSep — простой join без strings package (минимизировать imports в UI файле).
func joinSep(parts []string, sep string) string {
	out := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if out != "" {
			out += sep
		}
		out += p
	}
	return out
}
