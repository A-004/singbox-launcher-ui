// Package models содержит модели данных конфигуратора.
//
// Файл preset_ref_state.go — модель preset-ref правила (SPEC 053, kind=preset
// в state.json v6). Параллельно RuleState (который покрывает legacy
// kind=inline/srs).
//
// Preset-ref — это тонкая ссылка `{Ref → template.preset.id, Vars → diff от template defaults}`.
// Match-поля живут в template, расширяются при build через preset_expand.go.
package models

// PresetRefState — UI state одного preset-ref правила.
type PresetRefState struct {
	// Ref — id template-preset'а (template.presets[i].id).
	Ref string

	// Enabled — включено ли правило.
	Enabled bool

	// Vars — пользовательские значения переменных, только diff от template defaults.
	// Пустая map = всё дефолтное. Bump RequiredTemplateRef → новые дефолты подтягиваются автоматически.
	Vars map[string]string
}

// Clone — глубокая копия (для diff/undo сценариев).
func (p *PresetRefState) Clone() *PresetRefState {
	if p == nil {
		return nil
	}
	cp := &PresetRefState{
		Ref:     p.Ref,
		Enabled: p.Enabled,
		Vars:    make(map[string]string, len(p.Vars)),
	}
	for k, v := range p.Vars {
		cp.Vars[k] = v
	}
	return cp
}
