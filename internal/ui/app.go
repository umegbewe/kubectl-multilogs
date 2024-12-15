package ui

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/umegbewe/kubectl-multilog/internal/controller"
	"github.com/umegbewe/kubectl-multilog/internal/model"
)

type App struct {
	App              *tview.Application
	layout           *tview.Flex
	hierarchy        *tview.TreeView
	logTextView      *ScrollableTextView
	searchInput      *tview.InputField
	statusBar        *tview.TextView
	controller       *controller.Controller
	model            *model.Model
	clusterDropdown  *tview.DropDown
	liveTailBtn      *tview.Button
	caseSensitiveBtn *tview.Button
	wholeWordBtn     *tview.Button
	regexBtn         *tview.Button
	prevMatchBtn     *tview.Button
	nextMatchBtn     *tview.Button
	matchCountText   *tview.TextView
	searchTimer      *time.Timer
	tabBar           *tview.Flex
	logPages         *tview.Pages
	tabs             []*Tab
	activeTab        int
	contextPages     *tview.Pages
	mutex            sync.Mutex

	namespaces       []*model.Namespace
	pods             []*model.Pod
}

func LogExplorerTUI(model *model.Model) *App {
	ctrl := controller.NewController(model.K8sClient)
	tui := &App{
		App:         tview.NewApplication(),
		model:       model,
		controller:  ctrl,
		layout:      tview.NewFlex(),
		hierarchy:   tview.NewTreeView().SetGraphics(false),
		searchInput: tview.NewInputField().SetLabel("Search: ").SetLabelColor(colors.Accent),
		statusBar:   tview.NewTextView().SetTextAlign(tview.AlignLeft),
	}

	if err := tui.setupUI(); err != nil {
		log.Fatalf("Failed to set up UI: %v", err)
	}

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

	t.contextPages = tview.NewPages()
	t.contextPages.AddPage("main", t.layout, true, true)
	t.App.SetRoot(t.contextPages, true)

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
			t.controller.Stop()
			t.App.Stop()
			return nil
		}
		return event
	})

	go func() {
		namespace := make(chan []*model.Namespace, 1)
		t.controller.RegisterNamespaceObserver(namespace)
		go t.handleNamespaceUpdates(namespace)
	
		pod := make(chan []*model.Pod, 1) 
		t.controller.RegisterPodObserver(pod)
		go t.handlePodUpdates(pod)
	}()
	
	return t.App.Run()
}
