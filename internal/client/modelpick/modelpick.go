// Package modelpick is the model picker: the popup that chooses the model
// and the reasoning effort a chat's next turn uses. Providers are on the
// left, their models on the right, and the effort row below. Typing
// filters the models.
package modelpick

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/pkg/api"
)

// focus is the column that takes the arrow keys.
type focus int

// The columns, in Tab order.
const (
	focusProviders focus = iota
	focusModels
	focusEffort
)

// maxFilter keeps a stray paste from growing without bound.
const maxFilter = 64

// Model is one open model picker.
type Model struct {
	current string // the chat's model, "provider/model", or ""

	providers []api.ProviderModels
	provider  int // index in providers
	model     int // index in shown()
	level     int // 0 for the provider's default, else 1 + the effort's index
	focus     focus
	filter    []rune
	note      string

	ready   bool // SetList was called
	touched bool // the user moved; a new list keeps their place
}

// New opens a picker on the chat's current model and effort. Both may be
// empty, as before the first model is chosen.
func New(model, effort string) *Model {
	return &Model{current: model, level: effortIndex(effort), focus: focusModels}
}

// SetList replaces the providers and their models. The chat's current
// model is selected once it appears; after the user moves, a new list
// keeps their place.
func (m *Model) SetList(providers []api.ProviderModels) {
	providerName, modelID := m.providerName(), m.modelID()
	if !m.ready || !m.touched {
		providerName, modelID = modelProvider(m.current), modelIDOf(m.current)
	}
	m.providers, m.ready = providers, true
	m.provider = m.findProvider(providerName)
	m.model = m.findModel(modelID)
	m.clampLevel()
}

// findProvider returns the index of the provider called name, else the
// first connected one, else the first.
func (m *Model) findProvider(name string) int {
	if i := slices.IndexFunc(m.providers, func(p api.ProviderModels) bool { return p.Name == name }); i >= 0 {
		return i
	}
	if i := slices.IndexFunc(m.providers, func(p api.ProviderModels) bool { return p.Connected }); i >= 0 {
		return i
	}
	return 0
}

// findModel returns the index of the model called id among the shown
// models, or 0.
func (m *Model) findModel(id string) int {
	if i := slices.IndexFunc(m.shown(), func(mi api.ModelInfo) bool { return mi.ID == id }); i >= 0 {
		return i
	}
	return 0
}

// Key handles one key press.
func (m *Model) Key(k tea.Key) picker.Result {
	m.note = ""
	switch {
	case k.Code == tea.KeyEscape:
		return picker.Cancelled
	case k.Code == tea.KeyEnter:
		return m.accept(false)
	case k.Code == 'd' && k.Mod == tea.ModCtrl:
		return m.accept(true)
	case k.Code == tea.KeyTab && k.Mod == tea.ModShift:
		m.focus = (m.focus + 2) % 3
	case k.Code == tea.KeyTab:
		m.focus = (m.focus + 1) % 3
	case k.Code == tea.KeyUp:
		m.move(-1)
	case k.Code == tea.KeyDown:
		m.move(1)
	case k.Code == tea.KeyLeft && m.focus == focusEffort:
		m.stepEffort(-1)
	case k.Code == tea.KeyRight && m.focus == focusEffort:
		m.stepEffort(1)
	case k.Code == tea.KeyBackspace:
		m.backspace()
	case k.Code == 'u' && k.Mod == tea.ModCtrl:
		m.setFilter(nil)
	default:
		if text, ok := keys.Typed(k); ok {
			m.setFilter(append(m.filter, []rune(text)...))
		}
	}
	return picker.Open
}

// accept returns the result of choosing the selected model. A picker with
// no model to choose stays open and says so.
func (m *Model) accept(save bool) picker.Result {
	if _, ok := m.selected(); !ok {
		m.note = "No model to choose."
		return picker.Open
	}
	if save {
		return picker.Saved
	}
	return picker.Accepted
}

// Choice returns the selected model as "provider/model" and the effort.
// The effort is empty when the provider chooses. It returns empty strings
// when no model is selected.
func (m *Model) Choice() (model, effort string) {
	mi, ok := m.selected()
	if !ok {
		return "", ""
	}
	return m.providerName() + "/" + mi.ID, m.levelEffort()
}

// move moves the selection in the focused column.
func (m *Model) move(delta int) {
	switch m.focus {
	case focusProviders:
		m.setProvider(picker.Move(m.provider, delta, len(m.providers)))
	case focusModels:
		m.setModel(picker.Move(m.model, delta, len(m.shown())))
	}
}

// setProvider shows the models of provider i. The chat's model is chosen
// again when it belongs to that provider.
func (m *Model) setProvider(i int) {
	if i < 0 || i >= len(m.providers) || i == m.provider {
		return
	}
	m.provider, m.model, m.touched = i, 0, true
	if m.providerName() == modelProvider(m.current) {
		m.model = m.findModel(modelIDOf(m.current))
	}
	m.clampLevel()
}

// setModel selects model i and returns the effort to one the model takes.
func (m *Model) setModel(i int) {
	if i < 0 || i >= len(m.shown()) || i == m.model {
		return
	}
	m.model, m.touched = i, true
	m.clampLevel()
}

// stepEffort moves the effort by delta, skipping levels the model does not
// take.
func (m *Model) stepEffort(delta int) {
	n := len(api.EffortLevels) + 1
	for step := 1; step <= n; step++ {
		i := picker.Move(m.level, delta*step, n)
		if m.levelEnabled(i) {
			m.level, m.touched = i, true
			return
		}
	}
}

// backspace removes the filter's last rune.
func (m *Model) backspace() {
	if len(m.filter) == 0 {
		return
	}
	m.filter = m.filter[:len(m.filter)-1]
	m.model, m.touched = 0, true
	m.clampLevel()
}

// setFilter replaces the filter, keeping it at most maxFilter runes.
func (m *Model) setFilter(f []rune) {
	if len(f) > maxFilter {
		f = f[:maxFilter]
	}
	m.filter = f
	m.model, m.touched, m.focus = 0, true, focusModels
	m.clampLevel()
}

// shown returns the selected provider's models that match the filter.
func (m *Model) shown() []api.ModelInfo {
	if m.provider < 0 || m.provider >= len(m.providers) {
		return nil
	}
	all := m.providers[m.provider].Models
	f := strings.ToLower(strings.TrimSpace(string(m.filter)))
	if f == "" {
		return all
	}
	out := make([]api.ModelInfo, 0, len(all))
	for _, mi := range all {
		if strings.Contains(strings.ToLower(mi.ID), f) || strings.Contains(strings.ToLower(mi.DisplayName), f) {
			out = append(out, mi)
		}
	}
	return out
}

// selected returns the model under the pointer.
func (m *Model) selected() (api.ModelInfo, bool) {
	shown := m.shown()
	if m.model < 0 || m.model >= len(shown) {
		return api.ModelInfo{}, false
	}
	return shown[m.model], true
}

// levelEnabled reports whether the model takes effort level i. The default
// level always applies.
func (m *Model) levelEnabled(i int) bool {
	if i == 0 {
		return true
	}
	mi, ok := m.selected()
	return ok && mi.EffortSupported(api.EffortLevels[i-1])
}

// clampLevel returns the effort to one the selected model takes. With no
// model selected yet, it keeps the effort until a model arrives.
func (m *Model) clampLevel() {
	if _, ok := m.selected(); ok && !m.levelEnabled(m.level) {
		m.level = 0
	}
}

// levelEffort returns the effort of level i, or "" for the default.
func (m *Model) levelEffort() string {
	if m.level <= 0 || m.level > len(api.EffortLevels) {
		return ""
	}
	return api.EffortLevels[m.level-1]
}

// effortIndex returns 1 + the index of effort, or 0 when it is empty or
// unknown.
func effortIndex(effort string) int {
	if i := slices.Index(api.EffortLevels[:], effort); i >= 0 {
		return i + 1
	}
	return 0
}

// providerName returns the selected provider's name, or "".
func (m *Model) providerName() string {
	if m.provider < 0 || m.provider >= len(m.providers) {
		return ""
	}
	return m.providers[m.provider].Name
}

// modelID returns the selected model's ID, or "".
func (m *Model) modelID() string {
	mi, ok := m.selected()
	if !ok {
		return ""
	}
	return mi.ID
}

// modelProvider returns the provider part of a model name, or "".
func modelProvider(name string) string {
	p, _, _ := api.ParseModelName(name)
	return p
}

// modelIDOf returns the model part of a model name, or "".
func modelIDOf(name string) string {
	_, id, _ := api.ParseModelName(name)
	return id
}
