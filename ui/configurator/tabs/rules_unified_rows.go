// File rules_unified_rows.go — единый renderer строк правил для Rules tab.
//
// Обходит model.RuleOrder в порядке слотов; для каждого slot dispatch'ит:
//   - SlotKindCustom → существующий tile builder из rules_tab.go (rules_box.Add)
//     для одного CustomRule (legacy inline/srs)
//   - SlotKindPresetRef → preset-ref tile (см. ниже)
//
// Drag ↑↓ и delete действуют на индексы RuleOrder, не на CustomRules/PresetRefs.
// При delete юзер видит как пропадает конкретный tile; CompactRuleOrderIndices
// поддерживает целостность ссылок на сдвинувшиеся индексы.
package tabs

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	ttwidget "github.com/dweymouth/fyne-tooltip/widget"

	"singbox-launcher/core/services"
	wizardtemplate "singbox-launcher/core/template"
	"singbox-launcher/internal/constants"
	"singbox-launcher/internal/debuglog"
	"singbox-launcher/internal/dialogs"
	"singbox-launcher/internal/fynewidget"
	"singbox-launcher/internal/locale"
	wizardmodels "singbox-launcher/ui/configurator/models"
	wizardpresentation "singbox-launcher/ui/configurator/presentation"
)

// buildUnifiedRuleRows — один обход через RuleOrder, рендерит tile per slot.
func buildUnifiedRuleRows(
	presenter *wizardpresentation.WizardPresenter,
	model *wizardmodels.WizardModel,
	guiState *wizardpresentation.GUIState,
	availableOutbounds []string,
	showAddRuleDialog ShowAddRuleDialogFunc,
	rulesBox *fyne.Container,
) {
	for slotIdx, slot := range model.RuleOrder {
		switch slot.Kind {
		case wizardmodels.SlotKindCustom:
			if slot.Index < 0 || slot.Index >= len(model.CustomRules) {
				continue
			}
			buildSingleCustomRuleRow(presenter, model, guiState, availableOutbounds, showAddRuleDialog, rulesBox, slot.Index, slotIdx)
		case wizardmodels.SlotKindPresetRef:
			if slot.Index < 0 || slot.Index >= len(model.PresetRefs) {
				continue
			}
			buildSinglePresetRefRow(presenter, model, guiState, showAddRuleDialog, rulesBox, slot.Index, slotIdx)
		}
	}
}

// buildSinglePresetRefRow рисует tile для одного preset-ref'а (kind=preset).
// Tile match что у CustomRule tile: drag ↑↓ / enable / label+summary / edit / delete.
// Drag оперирует индексами model.RuleOrder.
func buildSinglePresetRefRow(
	presenter *wizardpresentation.WizardPresenter,
	model *wizardmodels.WizardModel,
	guiState *wizardpresentation.GUIState,
	showAddRuleDialog ShowAddRuleDialogFunc,
	rulesBox *fyne.Container,
	refIdx int,
	slotIdx int,
) {
	pr := model.PresetRefs[refIdx]

	// Lookup template preset for label / vars schema (broken-preset → marker).
	var tplPreset *wizardtemplate.Preset
	if model.TemplateData != nil {
		for i := range model.TemplateData.Presets {
			if model.TemplateData.Presets[i].ID == pr.Ref {
				tplPreset = &model.TemplateData.Presets[i]
				break
			}
		}
	}

	labelText, brokenRef := presetTileLabel(pr, tplPreset)

	label := widget.NewLabel(labelText)
	label.Truncation = fyne.TextTruncateEllipsis

	enableCh := widget.NewCheck("", func(on bool) {
		pr.Enabled = on
		presenter.MarkAsChanged()
		model.TemplatePreviewNeedsUpdate = true
		// Бандлd DNS-серверы и dns_rule preset'а появляются/исчезают вместе с
		// enable toggle (renderPresetBundledDNSRows пропускает !pr.Enabled).
		// Без RefreshDNSListAndSelects юзер видит stale entries в DNS tab.
		presenter.RefreshDNSListAndSelects()
	})
	enableCh.SetChecked(pr.Enabled)
	if brokenRef {
		enableCh.Disable()
	}

	editBtn := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() {
		showEditPresetRefDialog(presenter, model, guiState, refIdx, showAddRuleDialog)
	})
	editBtn.Importance = widget.LowImportance
	if brokenRef {
		editBtn.Disable()
	}

	delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		dialog.ShowConfirm(
			"Confirmation",
			fmt.Sprintf("Delete preset '%s'?", labelText),
			func(ok bool) {
				if !ok {
					return
				}
				deletedIdx := refIdx
				model.PresetRefs = append(model.PresetRefs[:deletedIdx], model.PresetRefs[deletedIdx+1:]...)
				wizardmodels.CompactRuleOrderIndices(model, wizardmodels.SlotKindPresetRef, deletedIdx)
				model.TemplatePreviewNeedsUpdate = true
				presenter.MarkAsChanged()
				refreshRulesTabFromPresenter(presenter, showAddRuleDialog)
			},
			guiState.Window,
		)
	})
	delBtn.Importance = widget.LowImportance

	upBtn := widget.NewButton("↑", func() {
		moveSlotUp(presenter, model, slotIdx, showAddRuleDialog)
	})
	upBtn.Importance = widget.LowImportance
	if slotIdx <= 0 {
		upBtn.Disable()
	}

	downBtn := widget.NewButton("↓", func() {
		moveSlotDown(presenter, model, slotIdx, showAddRuleDialog)
	})
	downBtn.Importance = widget.LowImportance
	if slotIdx >= len(model.RuleOrder)-1 {
		downBtn.Disable()
	}

	// SRS-облачко: показываем если preset (с учётом текущих vars) содержит
	// remote rule_set'ы которые ещё не скачаны. На клик — скачивание всех
	// remote rule_set'ов preset'а через services.DownloadSRSGroup.
	// Когда юзер выключит var управляющий remote rule_set (например geoip_enabled)
	// → presetRefSRSEntries вернёт пустой list → облачко исчезнет.
	var srsBtn *ttwidget.Button
	if entries := presetRefSRSEntries(pr, tplPreset); len(entries) > 0 && model.ExecDir != "" {
		srsBtn = makePresetSRSButton(presenter, model, guiState, entries, showAddRuleDialog)
	}

	leftCluster := container.NewHBox(upBtn, downBtn, fynewidget.CheckLeadingWrap(enableCh))
	var rightCluster *fyne.Container
	if srsBtn != nil {
		rightCluster = container.NewHBox(srsBtn, editBtn, delBtn)
	} else {
		rightCluster = container.NewHBox(editBtn, delBtn)
	}
	row := container.NewBorder(nil, nil, leftCluster, rightCluster, label)
	rulesBox.Add(row)
}

// makePresetSRSButton — облачко скачивания remote rule_set'ов preset'а.
// Текст кнопки: "Download" если не все скачаны, "Downloaded ✓" если все есть.
// На клик — параллельный download с timeout, status update после завершения.
func makePresetSRSButton(
	presenter *wizardpresentation.WizardPresenter,
	model *wizardmodels.WizardModel,
	guiState *wizardpresentation.GUIState,
	entries []services.SRSEntry,
	showAddRuleDialog ShowAddRuleDialogFunc,
) *ttwidget.Button {
	initialText := locale.T("wizard.rules.button_srs_download")
	if services.AllSRSDownloadedForEntries(model.ExecDir, entries) {
		initialText = locale.T("wizard.rules.button_srs_done")
	}
	btn := ttwidget.NewButton(initialText, nil)
	btn.Importance = widget.LowImportance
	tipURLs := make([]string, 0, len(entries))
	for _, e := range entries {
		tipURLs = append(tipURLs, e.URL)
	}
	if len(tipURLs) > 0 {
		btn.SetToolTip(strings.Join(tipURLs, "\n"))
	}
	btn.OnTapped = func() {
		btn.Disable()
		btn.SetText(locale.T("wizard.rules.button_srs_loading"))
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			err := services.DownloadSRSGroup(ctx, model.ExecDir, entries)
			presenter.UpdateUI(func() {
				btn.Enable()
				if err != nil {
					btn.SetText(locale.T("wizard.rules.button_srs_download"))
					ruleSetsDir := filepath.Join(model.ExecDir, constants.BinDirName, constants.RuleSetsDirName)
					firstURL := ""
					if len(entries) > 0 {
						firstURL = entries[0].URL
					}
					debuglog.WarnLog("preset_srs: download failed: %v", err)
					dialogs.ShowDownloadFailedManual(guiState.Window, locale.T("wizard.rules.error_srs_failed"), firstURL, ruleSetsDir)
					return
				}
				btn.SetText(locale.T("wizard.rules.button_srs_done"))
				// Refresh tab to recompute "Downloaded ✓" state and re-emit config.
				model.TemplatePreviewNeedsUpdate = true
				presenter.MarkAsChanged()
				refreshRulesTabFromPresenter(presenter, showAddRuleDialog)
			})
		}()
	}
	return btn
}

// presetTileLabel — текст для tile preset-ref'а: label + non-default vars summary.
// Возвращает (text, brokenRef): brokenRef=true когда preset не найден в template.
func presetTileLabel(pr *wizardmodels.PresetRefState, tpl *wizardtemplate.Preset) (string, bool) {
	if tpl == nil {
		return fmt.Sprintf("⚠ Broken preset: %s", pr.Ref), true
	}
	labelText := tpl.Label
	if labelText == "" {
		labelText = tpl.ID
	}
	if summary := summarizePresetVarsCompact(pr, tpl); summary != "" {
		labelText += "  ·  " + summary
	}
	return labelText, false
}

func summarizePresetVarsCompact(pr *wizardmodels.PresetRefState, tpl *wizardtemplate.Preset) string {
	if pr == nil || tpl == nil || len(pr.Vars) == 0 {
		return ""
	}
	defaults := make(map[string]string, len(tpl.Vars))
	for _, v := range tpl.Vars {
		defaults[v.Name] = v.Default
	}
	keys := make([]string, 0, len(pr.Vars))
	for k := range pr.Vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v := pr.Vars[k]
		if v == "" || v == defaults[k] {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ", ")
}

// moveSlotUp / moveSlotDown — swap slots в RuleOrder.
func moveSlotUp(presenter *wizardpresentation.WizardPresenter, model *wizardmodels.WizardModel, slotIdx int, showAddRuleDialog ShowAddRuleDialogFunc) {
	if slotIdx <= 0 || slotIdx >= len(model.RuleOrder) {
		return
	}
	model.RuleOrder[slotIdx], model.RuleOrder[slotIdx-1] = model.RuleOrder[slotIdx-1], model.RuleOrder[slotIdx]
	model.TemplatePreviewNeedsUpdate = true
	presenter.MarkAsChanged()
	refreshRulesTabFromPresenter(presenter, showAddRuleDialog)
}

func moveSlotDown(presenter *wizardpresentation.WizardPresenter, model *wizardmodels.WizardModel, slotIdx int, showAddRuleDialog ShowAddRuleDialogFunc) {
	if slotIdx < 0 || slotIdx >= len(model.RuleOrder)-1 {
		return
	}
	model.RuleOrder[slotIdx], model.RuleOrder[slotIdx+1] = model.RuleOrder[slotIdx+1], model.RuleOrder[slotIdx]
	model.TemplatePreviewNeedsUpdate = true
	presenter.MarkAsChanged()
	refreshRulesTabFromPresenter(presenter, showAddRuleDialog)
}
