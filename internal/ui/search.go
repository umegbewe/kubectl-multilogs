package ui

import (
	"fmt"
	"strings"

	"github.com/umegbewe/kubectl-multilog/internal/search"
)

func (t *App) performSearch(term string) {
	if t.activeTab == -1 || t.activeTab >= len(t.tabs) {
		t.setStatus("No active tab for search")
		return
	}

	activeTab := t.tabs[t.activeTab]
	activeTab.SearchTerm = term

	if term == "" {
		t.resetSearch()
		return
	}

	lines := activeTab.LogBuffer.GetLinesContent()
	options := activeTab.SearchOptions

	searchResult, err := search.PerformSearch(lines, term, options)
	if err != nil {
		t.setStatusError(fmt.Sprintf("Search error: %v", err))
		return
	}

	activeTab.SearchResult = searchResult
	activeTab.CurrentMatchIndex = 0

	if len(searchResult.Matches) > 0 {
		t.highlightMatches()
		t.updateSearchStatus()
		t.setupSearchNavigation()
	} else {
		t.resetSearch()
	}
}

func (t *App) highlightMatches() {
	if t.activeTab == -1 || t.activeTab >= len(t.tabs) {
		return
	}

	activeTab := t.tabs[t.activeTab]
	logTextView := activeTab.LogView

	logTextView.Clear()
	lines := activeTab.LogBuffer.GetLines()
	matchIndices := make(map[int][]*search.Match)

	for idx, match := range activeTab.SearchResult.Matches {
		lineMatches := matchIndices[match.LineNumber]
		match.Selected = idx == activeTab.CurrentMatchIndex
		lineMatches = append(lineMatches, match)
		matchIndices[match.LineNumber] = lineMatches
	}

	for lineNumber, line := range lines {
		content := line.Content
		if matches, ok := matchIndices[lineNumber]; ok {
			highlightedContent := highlightMatchesInLineWithSelection(content, matches)
			fmt.Fprintln(logTextView, highlightedContent)
		} else {
			fmt.Fprintln(logTextView, content)
		}
	}
}

func highlightMatchesInLineWithSelection(line string, matches []*search.Match) string {
	var result strings.Builder
	lastIndex := 0
	for _, match := range matches {
		result.WriteString(line[lastIndex:match.StartIndex])
		if match.Selected {
			result.WriteString("[#FF00FF]")
		} else {
			result.WriteString("[#00FF00]")
		}
		result.WriteString(line[match.StartIndex:match.EndIndex])
		result.WriteString("[-]")
		lastIndex = match.EndIndex
	}
	result.WriteString(line[lastIndex:])
	return result.String()
}

func (t *App) updateSearchForNewLogs() {
	if t.activeTab < 0 || t.activeTab >= len(t.tabs) {
		return
	}
	activeTab := t.tabs[t.activeTab]

	if activeTab.SearchResult == nil || activeTab.SearchResult.Term == "" {
		return
	}

	options := activeTab.SearchOptions
	lines := activeTab.LogBuffer.GetLinesContent()

	searchResult, err := search.PerformSearch(lines, activeTab.SearchResult.Term, options)
	if err != nil {
		t.setStatusError(fmt.Sprintf("Search error: %v", err))
		return
	}

	activeTab.SearchResult = searchResult
	t.highlightMatches()
	t.updateSearchStatus()
	t.setupSearchNavigation()
}

func (t *App) setupSearchNavigation() {
	if t.activeTab == -1 || t.activeTab >= len(t.tabs) {
		return
	}
	activeTab := t.tabs[t.activeTab]
	if activeTab.SearchResult == nil {
		t.prevMatchBtn.SetDisabled(true)
		t.nextMatchBtn.SetDisabled(true)
		t.matchCountText.SetText("")
		return
	}
	matchCount := len(activeTab.SearchResult.Matches)
	t.prevMatchBtn.SetDisabled(matchCount == 0)
	t.nextMatchBtn.SetDisabled(matchCount == 0)
	activeTab.CurrentMatchIndex = 0
	if matchCount > 0 {
		t.navigateToMatch(0)
	} else {
		t.matchCountText.SetText("No matches")
	}
}

func (t *App) navigateToMatch(direction int) {
	if t.activeTab == -1 || t.activeTab >= len(t.tabs) {
		return
	}
	activeTab := t.tabs[t.activeTab]
	matchCount := len(activeTab.SearchResult.Matches)
	if matchCount == 0 {
		return
	}

	activeTab.CurrentMatchIndex = (activeTab.CurrentMatchIndex + direction + matchCount) % matchCount
	t.highlightMatches()
	currentMatch := activeTab.SearchResult.Matches[activeTab.CurrentMatchIndex]
	activeTab.LogView.ScrollTo(currentMatch.LineNumber, 0)
	t.matchCountText.SetText(fmt.Sprintf("Match %d/%d", activeTab.CurrentMatchIndex+1, matchCount))
}

func (t *App) updateSearchStatus() {
	if t.activeTab < 0 || t.activeTab >= len(t.tabs) {
		return
	}
	activeTab := t.tabs[t.activeTab]
	if activeTab.SearchResult == nil {
		t.matchCountText.SetText("")
		return
	}
	matchCount := len(activeTab.SearchResult.Matches)
	t.matchCountText.SetText(fmt.Sprintf("%d matches", matchCount))
	t.setStatus(fmt.Sprintf("Found %d matches for '%s'", matchCount, activeTab.SearchResult.Term))
}

func (t *App) resetSearch() {
	if t.activeTab < 0 || t.activeTab >= len(t.tabs) {
		return
	}
	activeTab := t.tabs[t.activeTab]
	activeTab.SearchTerm = ""
	activeTab.SearchResult = nil
	activeTab.CurrentMatchIndex = 0
	activeTab.LogView.Clear()
	activeTab.LogView.SetText(strings.Join(t.getVisibleLogLines(), "\n"))
	activeTab.LogView.ScrollToEnd()
	t.setStatus("Search cleared")
	t.matchCountText.SetText("")
	t.prevMatchBtn.SetDisabled(true)
	t.nextMatchBtn.SetDisabled(true)
}

func (t *App) getVisibleLogLines() []string {
	if t.activeTab >= 0 && t.activeTab < len(t.tabs) {
		activeTab := t.tabs[t.activeTab]
		lines := activeTab.LogBuffer.GetLines()
		visibleLines := make([]string, len(lines))
		for i, line := range lines {
			visibleLines[i] = line.Content
		}
		return visibleLines
	}
	return []string{}
}
