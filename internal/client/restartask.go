package client

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/confirm"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

func (a *app) askRestart(mismatch *api.VersionMismatchError) {
	a.restartAsk = mismatch
	title, question := restartQuestion(mismatch)
	a.restartPopup = confirm.New(title, question, "Restart")
}

func (a *app) restartResult(result confirm.Result) tea.Cmd {
	if result == confirm.Open {
		return nil
	}
	a.restartPopup = nil
	if result == confirm.Yes {
		a.result.Restart = true
		return tea.Quit
	}
	if a.restartAsk != nil && a.restartAsk.ClientProtocol != a.restartAsk.ServerProtocol {
		a.result.Err = a.restartAsk
		return tea.Quit
	}
	return nil
}

func (a *app) restartView(content string) string {
	if a.restartPopup == nil {
		return content
	}
	box, x, y := a.restartPopup.View(a.theme, a.cols, a.rows)
	return ui.Overlay(content, box, x, y)
}
