package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	k8s "github.com/umegbewe/kubectl-multilog/internal/k8sclient"
	"github.com/umegbewe/kubectl-multilog/internal/search"
)

type Tab struct {
	Namespace         string
	Pod               string
	Container         string
	LogView           *ScrollableTextView
	TabFlex           *tview.Flex
	TabButton         *tview.Button
	LogBuffer         *search.LogBuffer
	SearchResult      *search.SearchResult
	CurrentMatchIndex int
	SearchTerm        string
	SearchOptions     search.SearchOptions
}

func (t *App) initTabs() {
	t.tabBar = tview.NewFlex()
	t.logPages = tview.NewPages()
	t.tabs = []*Tab{}
	t.activeTab = 0
}

func (t *App) addNewTab(namespace, pod, container string) {
	scrollBar := NewScrollBar()
	logTextView := NewScrollableTextView(t.App, scrollBar, func() {
		t.updateScrollBar()
	})
	logTextView.SetDynamicColors(true)
	logTextView.SetRegions(true)
	logTextView.SetScrollable(true)
	logTextView.SetWordWrap(true)
	logTextView.SetBackgroundColor(colors.Background)
	logTextView.SetTitle("Logs")
	logTextView.SetTitleColor(colors.Accent)
	logTextView.SetBorder(true)
	logTextView.SetBorderColor(colors.TopBar)
	logTextView.SetBorderAttributes(tcell.AttrDim)

	logViewContainer := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(logTextView, 0, 1, true).
		AddItem(scrollBar, 1, 0, false)

	tab := &Tab{
		Namespace:         namespace,
		Pod:               pod,
		Container:         container,
		LogView:           logTextView,
		LogBuffer:         search.NewLogBuffer(),
		TabButton:         nil,
		SearchOptions:     search.SearchOptions{},
		SearchTerm:        "",
		SearchResult:      nil,
		CurrentMatchIndex: 0,
	}

	t.tabs = append(t.tabs, tab)

	t.logPages.AddPage(tab.Pod+"-"+tab.Container, logViewContainer, true, len(t.tabs) == 1)

	tabButton := tview.NewButton(fmt.Sprintf("%s/%s", pod, container))
	tabButton.SetSelectedFunc(func() {
		idx := t.getTabIndex(namespace, pod, container)
		if idx != -1 {
			t.switchToTab(idx)
		}
	})

	tabButton.SetStyle(tcell.StyleDefault.Background(colors.TopBar))
	tabButton.SetLabelColor(colors.Text)
	tab.TabButton = tabButton

	closeButton := tview.NewButton("x")
	closeButton.SetSelectedFunc(func() {
		t.closeTab(tab)
	})
	closeButton.SetStyle(tcell.StyleDefault.Background(colors.TopBar))
	closeButton.SetLabelColor(colors.Text)

	tabFlex := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(tabButton, 0, 1, false).
		AddItem(closeButton, 3, 0, false)
	tabFlex.SetBackgroundColor(colors.TopBar)
	tab.TabFlex = tabFlex

	t.tabBar.AddItem(tabFlex, 0, 1, false)
	t.adjustTabWidths()
}

func (t *App) switchToTab(index int) {
	if index < 0 || index >= len(t.tabs) {
		return
	}

	if t.activeTab >= 0 && t.activeTab < len(t.tabs) {
		t.tabs[t.activeTab].TabButton.SetLabelColor(tcell.ColorWhite)
	}

	t.activeTab = index
	activeTab := t.tabs[t.activeTab]

	activeTab.TabButton.SetLabelColor(colors.Accent)

	t.logPages.SwitchToPage(activeTab.Pod + "-" + activeTab.Container)

	activeTab.LogView.Clear()
	lines := activeTab.LogBuffer.GetLines()
	for _, line := range lines {
		fmt.Fprintln(activeTab.LogView, line.Content)
	}
	activeTab.LogView.ScrollToEnd()
	t.updateScrollBar()

	t.App.SetFocus(activeTab.LogView)

	t.statusBar.SetText(fmt.Sprintf("Viewing logs for %s/%s/%s", activeTab.Namespace, activeTab.Pod, activeTab.Container))

	t.searchInput.SetText(activeTab.SearchTerm)
	t.updateSearchOptionButtons()
	t.performSearch(activeTab.SearchTerm)
	t.updateSearchStatus()
	t.setupSearchNavigation()
}

func (t *App) getTabIndex(namespace, pod, container string) int {
	for i, tab := range t.tabs {
		if tab.Namespace == namespace && tab.Pod == pod && tab.Container == container {
			return i
		}
	}
	return -1
}

func (t *App) closeTab(tab *Tab) {
	idx := t.getTabIndex(tab.Namespace, tab.Pod, tab.Container)
	if idx == -1 {
		return
	}

	t.tabs = append(t.tabs[:idx], t.tabs[idx+1:]...)

	t.tabBar.RemoveItem(tab.TabFlex)
	t.logPages.RemovePage(tab.Pod + "-" + tab.Container)

	if len(t.tabs) == 0 {
		t.activeTab = -1
		t.statusBar.SetText("No tabs open")
		t.App.SetFocus(nil)
	} else {
		if t.activeTab >= len(t.tabs) {
			t.activeTab = len(t.tabs) - 1
		}
		t.switchToTab(t.activeTab)
	}

	t.adjustTabWidths()
	if t.activeTab == -1 || t.activeTab >= len(t.tabs) {
		t.searchInput.SetText("")
		t.updateSearchOptionButtons()
		t.matchCountText.SetText("")
		t.prevMatchBtn.SetDisabled(true)
		t.nextMatchBtn.SetDisabled(true)
	}
}

func (t *App) adjustTabWidths() {
	_, _, availableWidth, _ := t.tabBar.GetRect()
	numTabs := len(t.tabs)
	if numTabs == 0 || availableWidth == 0 {
		return
	}

	tabFlexWidth := availableWidth / numTabs
	if tabFlexWidth < 10 {
		tabFlexWidth = 10
	}

	closeButtonWidth := 3

	maxLabelWidth := tabFlexWidth - closeButtonWidth - 2
	for _, tab := range t.tabs {
		// truncate label text if necessary
		labelText := fmt.Sprintf("%s/%s", tab.Pod, tab.Container)
		if len(labelText) > maxLabelWidth {
			labelText = labelText[:maxLabelWidth-3] + "..."
		}

		tab.TabButton.SetLabel(labelText)
		t.tabBar.ResizeItem(tab.TabFlex, tabFlexWidth, 0)
	}
}

func (t *App) loadLogs(namespace, pod, container string) {
	if t.model.LogStreamCancel != nil {
		t.model.LogStreamCancel()
	}

	idx := t.getTabIndex(namespace, pod, container)
	if idx == -1 {
		// add new tab if it doesn't exist
		t.addNewTab(namespace, pod, container)
		idx = len(t.tabs) - 1
	}

	t.switchToTab(idx)
	activeTab := t.tabs[t.activeTab]

	t.showLoading(fmt.Sprintf("Loading logs for %s/%s/%s", namespace, pod, container))

	tail := int64(150)
	logs, logChan, err := t.model.K8sClient.GetLogs(namespace, pod, container, true, &tail)
	if err != nil {
		t.App.QueueUpdateDraw(func() {
			t.statusBar.SetText(fmt.Sprintf("Error fetching logs: %v", err))
		})
		return
	}

	t.App.QueueUpdateDraw(func() {
		activeTab.LogView.Clear()
		activeTab.LogBuffer.Clear()
		for _, line := range strings.Split(logs, "\n") {
			if line != "" {
				activeTab.LogBuffer.AddLine(line)
				fmt.Fprintf(activeTab.LogView, "%s\n", line)
			}
		}
		activeTab.LogView.ScrollToEnd()
		t.updateScrollBar()
	})

	var ctx context.Context
	ctx, t.model.LogStreamCancel = context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case logEntry, ok := <-logChan:
				if !ok {
					return
				}
				t.processNewLogEntry(namespace, pod, container, logEntry)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (t *App) processNewLogEntry(namespace, pod, container, logEntry string) {
	idx := t.getTabIndex(namespace, pod, container)
	if idx == -1 {
		return
	}

	tab := t.tabs[idx]
	tab.LogBuffer.AddLine(logEntry)

	t.App.QueueUpdateDraw(func() {
		if t.activeTab == idx {
			fmt.Fprintf(tab.LogView, "%s\n", logEntry)
			// tab.LogView.ScrollToEnd()
			t.updateScrollBar()
		}
	})
}

// func (t *App) processLiveLogs() {
// 	for logEntry := range t.Model.LogChan {
// 		t.Model.LogMutex.Lock()
// 		if logEntry.Timestamp.After(t.Model.LiveTailStartTime) {
// 			formattedLog := fmt.Sprintf("[%s] [%s/%s/%s] %s: %s\n",
// 				logEntry.Timestamp.Format(time.RFC3339),
// 				logEntry.Namespace,
// 				logEntry.Pod,
// 				logEntry.Container,
// 				logEntry.Level,
// 				logEntry.Message)
// 			t.processNewLogEntry(formattedLog)
// 		}
// 		t.Model.LogMutex.Unlock()
// 	}
// }

func (t *App) clearLogView() {
	t.App.QueueUpdateDraw(func() {
		if t.activeTab >= 0 && t.activeTab < len(t.tabs) {
			activeTab := t.tabs[t.activeTab]
			activeTab.LogView.Clear()
			activeTab.LogView.SetText("")
		}
	})
}

func (t *App) toggleLiveTail() {
	if t.model.LiveTailActive {
		t.stopLiveTail()
		t.liveTailBtn.SetLabel("Start Live Tail").SetBackgroundColor(colors.TopBar)
	} else {
		t.startLiveTail()
		t.liveTailBtn.SetLabel("Stop Live Tail").SetBackgroundColor(colors.Accent)
	}
}

func (t *App) startLiveTail() {
	t.model.LiveTailActive = true
	t.logTextView.Clear()
	t.statusBar.SetText("Live tail active")

	t.model.LiveTailStartTime = time.Now()
	t.model.LiveTailCtx, t.model.LiveTailCancel = context.WithCancel(context.Background())
	t.model.LogChan = make(chan k8s.LogEntry, 100)

	go t.model.K8sClient.StreamAllLogs(t.model.LiveTailCtx, t.model.LogChan, t.model.LiveTailStartTime)
	// go t.processLiveLogs()
}

func (t *App) stopLiveTail() {
	t.model.LiveTailActive = false
	t.model.LiveTailCancel()
	close(t.model.LogChan)
	t.statusBar.SetText("Live tail stopped")
}
