package mdsync

import (
	"bytes"
	"io"
)

type filterState struct {
	ref           *snippetRef
	started       bool
	ended         bool
	startSkipLeft int
	endMatchCount int
	skipUntilLine int
	inSkipBetween bool
	curSkipEnd    *compiledPattern
	lines         []lineRange
	minIndent     int
	hasContent    bool
	ringBuf       []lineRange
	ringSize      int
}

type lineRange struct {
	start int
	end   int
}

func newFilterState(r *snippetRef) *filterState {
	s := &filterState{
		ref:           r,
		started:       r.startAt == nil,
		skipUntilLine: -1,
		minIndent:     -1,
	}
	if r.startAt != nil && r.startOffset < 0 {
		s.ringSize = -r.startOffset
	}
	return s
}

func (s *filterState) processLine(line []byte, lineStart, lineEnd, lineNum int, content []byte) {
	if s.ended {
		return
	}

	if !s.started {
		if s.ref.startAt.matches(line) {
			s.started = true
			if s.ref.startOffset > 0 {
				s.startSkipLeft = s.ref.startOffset
			} else if s.ref.startOffset < 0 {
				s.flushRingBuffer(content)
			}
			return
		}

		if s.ringSize > 0 {
			if len(s.ringBuf) == s.ringSize {
				copy(s.ringBuf, s.ringBuf[1:])
				s.ringBuf[s.ringSize-1] = lineRange{start: lineStart, end: lineEnd}
			} else {
				s.ringBuf = append(s.ringBuf, lineRange{start: lineStart, end: lineEnd})
			}
		}
		return
	}

	if s.startSkipLeft > 0 {
		s.startSkipLeft--
		return
	}

	if s.ref.endAt != nil && s.ref.endAt.matches(line) {
		if s.ref.endOffset == 0 {
			s.ended = true
			return
		}
		s.endMatchCount++
		if s.endMatchCount > s.ref.endOffset {
			s.ended = true
			return
		}
	}

	if !s.inSkipBetween {
		for i := range s.ref.skipBetween {
			if s.ref.skipBetween[i].start.matches(line) {
				s.inSkipBetween = true
				s.curSkipEnd = &s.ref.skipBetween[i].end
				break
			}
		}
	}
	if s.inSkipBetween {
		if s.curSkipEnd.matches(line) {
			s.inSkipBetween = false
			s.curSkipEnd = nil
		}
		return
	}

	if s.skipUntilLine >= lineNum {
		return
	}
	for _, sa := range s.ref.skipAfter {
		if sa.pattern.matches(line) {
			s.skipUntilLine = lineNum + sa.count
			break
		}
	}
	if s.skipUntilLine >= lineNum {
		return
	}

	for i := range s.ref.skipMatch {
		if s.ref.skipMatch[i].matches(line) {
			return
		}
	}

	s.acceptLine(line, lineStart, lineEnd)
}

func (s *filterState) acceptLine(line []byte, lineStart, lineEnd int) {
	s.lines = append(s.lines, lineRange{start: lineStart, end: lineEnd})

	trimmed := bytes.TrimSpace(line)
	if len(trimmed) > 0 {
		indent := 0
		for _, b := range line {
			if b == ' ' || b == '\t' {
				indent++
			} else {
				break
			}
		}
		if !s.hasContent || indent < s.minIndent {
			s.minIndent = indent
			s.hasContent = true
		}
	}
}

func (s *filterState) flushRingBuffer(content []byte) {
	for _, lr := range s.ringBuf {
		line := content[lr.start:lr.end]
		s.acceptLine(line, lr.start, lr.end)
	}
	s.ringBuf = s.ringBuf[:0]
}

func (s *filterState) writeTo(w io.Writer, content []byte) (bool, error) {
	if len(s.lines) == 0 {
		return false, nil
	}

	minIndent := max(0, s.minIndent)

	needNewline := false
	pendingNewlines := 0
	for _, lr := range s.lines {
		line := content[lr.start:lr.end]
		if len(bytes.TrimSpace(line)) == 0 {
			if needNewline {
				pendingNewlines++
			}
			continue
		}
		if needNewline {
			if _, err := w.Write([]byte{'\n'}); err != nil {
				return true, err
			}
			for range pendingNewlines {
				if _, err := w.Write([]byte{'\n'}); err != nil {
					return true, err
				}
			}
		}
		pendingNewlines = 0
		needNewline = true
		if len(line) > minIndent {
			if _, err := w.Write(line[minIndent:]); err != nil {
				return true, err
			}
		} else {
			if _, err := w.Write(line); err != nil {
				return true, err
			}
		}
	}

	return true, nil
}

