package ui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (t *App) initClusterDropdown(clusters []string) *tview.DropDown {
	return tview.NewDropDown().
		SetOptions(clusters, func(option string, index int) {
			if err := t.model.SwitchCluster(option); err != nil {
				t.setStatusError(fmt.Sprintf("Error switching cluster: %v", err))
				return
			}
			t.setStatus(fmt.Sprintf("Switched to cluster: %s", option))

			t.closeAllTabs()
			t.refreshHierarchy()
		}).
		SetCurrentOption(0).
		SetFieldWidth(20)
}

func (t *App) initTopBar() *tview.Flex {
	topBar := tview.NewFlex().SetDirection(tview.FlexColumn)

	t.clusterDropdown.SetLabel("Context: ")
	t.clusterDropdown.SetLabelColor(colors.Accent)
	t.clusterDropdown.SetFieldBackgroundColor(colors.TopBar)
	t.clusterDropdown.SetFieldTextColor(colors.Text)
	t.clusterDropdown.SetFieldWidth(100)
	t.clusterDropdown.SetBackgroundColor(colors.TopBar)
	t.clusterDropdown.SetListStyles(
		tcell.StyleDefault.Background(colors.Sidebar),
		tcell.StyleDefault.Background(colors.Highlight).Foreground(colors.Text),
	)

	t.searchInput.SetLabel(" Search: ").SetLabelColor(colors.Accent)
	t.searchInput.SetFieldBackgroundColor(colors.TopBar)
	t.searchInput.SetBackgroundColor(colors.TopBar)
	t.searchInput.SetFieldTextColor(colors.Text)

	t.caseSensitiveBtn = createButton("Aa", colors.Button, func() {
		t.toggleOption(&t.tabs[t.activeTab].SearchOptions.CaseSensitive)
	})
	t.wholeWordBtn = createButton("W", colors.Button, func() {
		t.toggleOption(&t.tabs[t.activeTab].SearchOptions.WholeWord)
	})
	t.regexBtn = createButton(".*", colors.Button, func() {
		t.toggleOption(&t.tabs[t.activeTab].SearchOptions.RegexEnabled)
	})

	t.prevMatchBtn = createButton("◀", colors.NavButton, func() {
		t.navigateToMatch(-1)
	})
	t.nextMatchBtn = createButton("▶", colors.NavButton, func() {
		t.navigateToMatch(1)
	})
	
	t.matchCountText = tview.NewTextView().SetTextAlign(tview.AlignRight)
	t.matchCountText.SetBackgroundColor(colors.TopBar)
	t.matchCountText.SetTextAlign(tview.AlignCenter)

	t.liveTailBtn = createButton("Live", colors.Button, func() {
		t.toggleLiveTail()
	})

	searchBar := tview.NewFlex().
		AddItem(t.searchInput, 0, 1, false).
		AddItem(tview.NewBox().SetBackgroundColor(colors.TopBar), 1, 0, false).
		AddItem(t.caseSensitiveBtn, 4, 0, false).
		AddItem(tview.NewBox().SetBackgroundColor(colors.TopBar), 1, 0, false).
		AddItem(t.wholeWordBtn, 3, 0, false).
		AddItem(tview.NewBox().SetBackgroundColor(colors.TopBar), 1, 0, false).
		AddItem(t.regexBtn, 4, 0, false).
		AddItem(tview.NewBox().SetBackgroundColor(colors.TopBar), 1, 0, false).
		AddItem(t.prevMatchBtn, 3, 0, false).
		AddItem(t.matchCountText, 12, 0, false).
		AddItem(t.nextMatchBtn, 3, 0, false)

	t.setupSearchHandler()

	topBar.AddItem(t.clusterDropdown, 0, 1, false)
	topBar.AddItem(searchBar, 0, 3, false)
	topBar.AddItem(t.liveTailBtn, 0, 1, false)

	return topBar
}

func (t *App) toggleOption(option *bool) {
	if t.activeTab == -1 || t.activeTab >= len(t.tabs) {
		return
	}

	*option = !*option
	t.updateSearchOptionButtons()
	t.performSearch(t.searchInput.GetText())
}

func (t *App) updateSearchOptionButtons() {
	if t.activeTab == -1 || t.activeTab >= len(t.tabs) {
		return
	}

	activeTab := t.tabs[t.activeTab]

	if activeTab.SearchOptions.CaseSensitive {
		t.caseSensitiveBtn.SetLabelColor(colors.Accent)
	} else {
		t.caseSensitiveBtn.SetLabelColor(colors.Text)
	}

	if activeTab.SearchOptions.WholeWord {
		t.wholeWordBtn.SetLabelColor(colors.Accent)
	} else {
		t.wholeWordBtn.SetLabelColor(colors.Text)
	}

	if activeTab.SearchOptions.RegexEnabled {
		t.regexBtn.SetLabelColor(colors.Accent)
	} else {
		t.regexBtn.SetLabelColor(colors.Text)
	}
}

func (t *App) setupSearchHandler() {
	t.searchInput.SetChangedFunc(func(text string) {
		if t.searchTimer != nil {
			t.searchTimer.Stop()
		}
		t.searchTimer = time.AfterFunc(200*time.Millisecond, func() {
			t.App.QueueUpdateDraw(func() {
				if t.activeTab == -1 || t.activeTab >= len(t.tabs) {
					return
				}
				activeTab := t.tabs[t.activeTab]
				activeTab.SearchTerm = text
				t.performSearch(text)
			})
		})
	})

	t.prevMatchBtn.SetSelectedFunc(func() {
		t.navigateToMatch(-1)
	})

	t.nextMatchBtn.SetSelectedFunc(func() {
		t.navigateToMatch(1)
	})
}