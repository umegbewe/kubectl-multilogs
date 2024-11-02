package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ScrollableTextView struct {
	*tview.TextView
	App           *tview.Application
	ScrollBar     *ScrollBar
	updateHandler func()
	onLineClick   func(lineNumber int)
}

func NewScrollableTextView(app *tview.Application, scrollBar *ScrollBar, updateHandler func()) *ScrollableTextView {
	return &ScrollableTextView{
		TextView:      tview.NewTextView(),
		App:           app,
		ScrollBar:     scrollBar,
		updateHandler: updateHandler,
	}
}

func (stv *ScrollableTextView) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	handler := stv.TextView.InputHandler()
	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		oldOffset, _ := stv.GetScrollOffset()
		if handler != nil {
			handler(event, setFocus)
		}
		newOffset, _ := stv.GetScrollOffset()
		if oldOffset != newOffset && stv.updateHandler != nil {
			stv.updateHandler()
		}
	}
}

func (stv *ScrollableTextView) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
	handler := stv.TextView.MouseHandler()
	return func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
		x, y := event.Position()
		if !stv.InRect(x, y) {
			return false, nil
		}

		switch action {
		case tview.MouseLeftClick:
			lineNumber := stv.getLineFromPosition(y)
            if stv.onLineClick != nil {
                stv.onLineClick(lineNumber)
            }
			return true, stv
		default:
			return handler(action, event, setFocus)
		}
	}
}

func (stv *ScrollableTextView) getLineFromPosition(y int) int {
	_, vy, _, _ := stv.GetInnerRect()
	scrollOffset, _ := stv.GetScrollOffset()
	lineNumber := scrollOffset + (y - vy)
	return lineNumber
}

func (stv *ScrollableTextView) SetOnLineClick(handler func(lineNumber int)) {
	stv.onLineClick = handler
}