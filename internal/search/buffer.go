package search

import (
	"sync"
)

const MaxLogLines = 10000

type LogLine struct {
	Content string
}

type LogBuffer struct {
	Lines    []LogLine
	mutex    sync.RWMutex
	maxLines int
}

func NewLogBuffer() *LogBuffer {
	return &LogBuffer{
		Lines:    make([]LogLine, 0, MaxLogLines),
		maxLines: MaxLogLines,
	}
}

func (lb *LogBuffer) AddLine(content string) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	lb.Lines = append(lb.Lines, LogLine{Content: content})
	
	if len(lb.Lines) > lb.maxLines {
		lb.Lines = lb.Lines[len(lb.Lines)-lb.maxLines:]
	}
}

func (lb *LogBuffer) GetLines() []LogLine {
	lb.mutex.RLock()
	defer lb.mutex.RUnlock()

	return lb.Lines
}

func (lb *LogBuffer) Clear() {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	lb.Lines = make([]LogLine, 0, lb.maxLines)
}

func (lb *LogBuffer) GetLinesContent() []string {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	contents := make([]string, len(lb.Lines))
	for i, line := range lb.Lines {
		contents[i] = line.Content
	}
	return contents
}
