package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (t *App) setStatus(message string) {
	t.statusBar.SetText(message)
}

func (t *App) setStatusError(message string) {
	t.statusBar.SetText("[red]" + message + "[-]")
}

func (t *App) showLoading(message string) {
	t.statusBar.SetText(message + " Loading...")

}

func createTreeNode(label string, isLeaf bool) *tview.TreeNode {
	node := tview.NewTreeNode("")

	if isLeaf {
		node.SetText(fmt.Sprintf("  %s", label))
	} else {
		node.SetText(fmt.Sprintf("▶ %s", label))
		node.SetExpanded(false)
	}

	return node
}

func setNodeWithToggleIcon(node *tview.TreeNode, label string, toggleFunc func()) {
	node.SetSelectedFunc(func() {
		if node.IsExpanded() {
			node.CollapseAll()
			node.SetText(fmt.Sprintf("▶ %s", label))
		} else {
			node.Expand()
			node.SetText(fmt.Sprintf("▼ %s", label))
			toggleFunc()
		}
	})
}

func createButton(label string, bgColor tcell.Color, selectedFunc func()) *tview.Button {
	return tview.NewButton(label).
		SetLabelColor(colors.Text).
		SetStyle(tcell.StyleDefault.Background(bgColor)).
		SetSelectedFunc(selectedFunc)
}

func (t *App) updateScrollBar() {
	if t.activeTab < 0 || t.activeTab >= len(t.tabs) {
		return
	}
	activeTab := t.tabs[t.activeTab]
	content := activeTab.LogView.GetText(true)

	lines := strings.Count(content, "\n")
	totalLines := lines
	if content != "" && content[len(content)-1] != '\n' {
		totalLines++
	}

	_, _, _, height := activeTab.LogView.GetInnerRect()
	firstVisibleLine, _ := activeTab.LogView.GetScrollOffset()

	activeTab.LogView.ScrollBar.SetTotalLines(totalLines)
	activeTab.LogView.ScrollBar.SetVisibleLines(height)
	activeTab.LogView.ScrollBar.SetCurrentLine(firstVisibleLine)
}

func (t *App) closeAllTabs() {
	for len(t.tabs) > 0 {
		t.closeTab(t.tabs[0])
	}
	t.activeTab = -1
	t.App.SetFocus(t.hierarchy)
}
