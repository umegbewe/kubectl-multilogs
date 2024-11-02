package ui

import (
	"github.com/rivo/tview"
	"github.com/gdamore/tcell/v2"
)

type Window struct {
    *tview.Box
    primitive     tview.Primitive
    content     tview.Primitive
    title       string
    Application *tview.Application
    pages       *tview.Pages
    pageID      string

    // window dimensions and position
    pos    position
    size   dimensions
    bounds screenBounds

    // interaction state
    isDragging    bool
    isResizing    bool
    dragStart     position
    resizeStart   position
    
    // configurable constants
    minSize dimensions
}

type position struct{ x, y int }
type dimensions struct{ width, height int }
type screenBounds struct{ maxX, maxY int }

const (
    defaultWidth  = 50
    defaultHeight = 15
    defaultX      = 10
    defaultY      = 5
    
    minWindowWidth  = 20
    minWindowHeight = 8 
)

func NewWindow(app *tview.Application, content tview.Primitive, title string, pages *tview.Pages, pageID string) *Window {
    win := &Window{
        Box:         tview.NewBox().SetBorder(true).SetBackgroundColor(tcell.ColorDefault).SetBorderAttributes(tcell.AttrBold).SetBorderColor(tcell.ColorBlue),
        content:     content,
        title:       title,
        Application: app,
        pages:       pages,
        pageID:      pageID,
        pos: position{
            x: defaultX,
            y: defaultY,
        },
        size: dimensions{
            width:  defaultWidth,
            height: defaultHeight,
        },
        minSize: dimensions{
            width:  minWindowWidth,
            height: minWindowHeight,
        },
    }
    
    win.Box.SetTitle(" " + title + " ")
    
    return win
}

func (w *Window) Draw(screen tcell.Screen) {
    maxX, maxY := screen.Size()
    w.bounds = screenBounds{maxX, maxY}
    
    w.constrainToScreen()
    
    w.SetRect(w.pos.x, w.pos.y, w.size.width, w.size.height)
    
    w.Box.Draw(screen)
    
    
    if w.hasDrawableContent() {
        innerX := w.pos.x + 1
        innerY := w.pos.y + 1
        innerWidth := w.size.width - 2
        innerHeight := w.size.height - 2
        
        w.content.SetRect(innerX, innerY, innerWidth, innerHeight)
        w.content.Draw(screen)
    }
}
func (w *Window) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
    return func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
        mouseX, mouseY := event.Position()
        mousePos := position{mouseX, mouseY}

        switch action {
        case tview.MouseLeftDown:
            if !w.InRect(mouseX, mouseY) {
                return false, nil
            }
            
            w.pages.SendToFront(w.pageID)
            setFocus(w)
            
            switch {
            case w.isOverCloseButton(mousePos):
                w.pages.RemovePage(w.pageID)
            case w.isOverTitleBar(mousePos):
                w.startDragging(mousePos)
            case w.isOverResizeHandle(mousePos):
                w.startResizing(mousePos)
            }
            return true, w
            
        case tview.MouseLeftUp:
            w.stopInteraction()
            return true, w
            
        case tview.MouseMove:
            if w.isDragging {
                w.handleDrag(mousePos)
            } else if w.isResizing {
                w.handleResize(mousePos)
            }
            return true, w
        }
        
        if w.content != nil {
            if handler := w.content.MouseHandler(); handler != nil {
                return handler(action, event, setFocus)
            }
        }
        
        return false, nil
    }
}

func (w *Window) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		if event.Key() == tcell.KeyEsc {
			// remove window from pages
			w.pages.RemovePage(w.pageID)
			return
		}
		if handler := w.primitive.InputHandler(); handler != nil {
			handler(event, setFocus)
		}
	}
}


func (w *Window) constrainToScreen() {
    if w.pos.x < 0 {
        w.pos.x = 0
    }
    if w.pos.y < 0 {
        w.pos.y = 0
    }
    if w.pos.x + w.size.width > w.bounds.maxX {
        w.pos.x = w.bounds.maxX - w.size.width
    }
    if w.pos.y + w.size.height > w.bounds.maxY {
        w.pos.y = w.bounds.maxY - w.size.height
    }
}

func (w *Window) hasDrawableContent() bool {
    return w.size.width > 2 && w.size.height > 2
}

func (w *Window) isOverTitleBar(p position) bool {
    return p.y == w.pos.y && p.x >= w.pos.x && p.x < w.pos.x+w.size.width-1
}

func (w *Window) isOverCloseButton(p position) bool {
    closeX := w.pos.x + w.size.width - 1
    return p.x == closeX && p.y == w.pos.y
}

func (w *Window) isOverResizeHandle(p position) bool {
    return p.x == w.pos.x+w.size.width-1 && p.y == w.pos.y+w.size.height-1
}

func (w *Window) startDragging(p position) {
    w.isDragging = true
    w.dragStart = p
}

func (w *Window) startResizing(p position) {
    w.isResizing = true
    w.resizeStart = p
}

func (w *Window) stopInteraction() {
    w.isDragging = false
    w.isResizing = false
}

func (w *Window) handleDrag(currentPos position) {
    if !w.isDragging {
        return
    }
    
    dx := currentPos.x - w.dragStart.x
    dy := currentPos.y - w.dragStart.y
    
    w.pos.x += dx
    w.pos.y += dy
    w.dragStart = currentPos
    
    w.constrainToScreen()
}

func (w *Window) handleResize(currentPos position) {
    if !w.isResizing {
        return
    }
    
    dx := currentPos.x - w.resizeStart.x
    dy := currentPos.y - w.resizeStart.y
    
    newWidth := w.size.width + dx
    newHeight := w.size.height + dy
    
    if newWidth >= w.minSize.width {
        w.size.width = newWidth
    }
    if newHeight >= w.minSize.height {
        w.size.height = newHeight
    }
    
    w.resizeStart = currentPos
    w.constrainToScreen()
}