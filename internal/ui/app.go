package ui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/umegbewe/kubectl-multilog/internal/model"
)

type App struct {
	App              *tview.Application
	layout           *tview.Flex
	hierarchy        *tview.TreeView
	logTextView      *ScrollableTextView
	searchInput      *tview.InputField
	statusBar        *tview.TextView
	clusterDropdown  *tview.DropDown
	liveTailBtn      *tview.Button
	caseSensitiveBtn *tview.Button
	wholeWordBtn     *tview.Button
	regexBtn         *tview.Button
	prevMatchBtn     *tview.Button
	nextMatchBtn     *tview.Button
	matchCountText   *tview.TextView
	searchTimer      *time.Timer
	model            *model.Model
	tabBar           *tview.Flex
	logPages         *tview.Pages
	tabs             []*Tab
	activeTab        int
}

func LogExplorerTUI(model *model.Model) *App {
	tui := &App{
		App:         tview.NewApplication(),
		layout:      tview.NewFlex(),
		hierarchy:   tview.NewTreeView().SetGraphics(false),
		searchInput: tview.NewInputField().SetLabel("Search: ").SetLabelColor(colors.Accent),
		statusBar:   tview.NewTextView().SetTextAlign(tview.AlignLeft),
		model:       model,
	}

	tui.setupUI()
	tui.refreshHierarchy()
	return tui
}

func (t *App) setupUI() error {
	t.initTabs()

	clusters := t.model.GetClusterNames()
	if len(clusters) == 0 {
		return fmt.Errorf("no clusters found")
	}

	t.clusterDropdown = t.initClusterDropdown(clusters)
	t.hierarchy.SetBackgroundColor(colors.Sidebar)
	t.statusBar.SetBackgroundColor(colors.TopBar)

	root := tview.NewTreeNode("Pods")
	t.hierarchy.SetRoot(root)

	topBar := t.initTopBar()

	mainContent := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(t.tabBar, 1, 0, false).
		AddItem(t.logPages, 0, 1, true)

	mainArea := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(t.hierarchy, 0, 2, false).
		AddItem(mainContent, 0, 5, true)

	t.layout = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(topBar, 1, 0, false).
		AddItem(mainArea, 0, 1, true).
		AddItem(t.statusBar, 1, 0, false)

	initialCluster := t.model.GetCurrentContext()
	for i, cluster := range clusters {
		if cluster == initialCluster {
			t.clusterDropdown.SetCurrentOption(i)
			break
		}
	}

	return nil
}

func (t *App) Run() error {
	t.App.EnableMouse(true)
	t.App.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			if t.model.LiveTailActive {
				t.stopLiveTail()
			}
			t.App.Stop()
			return nil
		}
		return event
	})

	return t.App.SetRoot(t.layout, true).Run()
}
