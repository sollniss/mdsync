package mdsync

import (
	"bytes"
	"strings"
	"testing"
)

func filterResult(s *filterState, content []byte) string {
	var buf strings.Builder
	s.writeTo(&buf, content)
	return buf.String()
}

func TestFilterStateBasic(t *testing.T) {
	content := []byte("line1\nline2\nline3\n")
	r := &snippetRef{sourceFile: "test.go"}
	state := newFilterState(r)

	lines := bytes.Split(content, []byte{'\n'})
	pos := 0
	for i, line := range lines[:len(lines)-1] { // skip empty last element
		lineEnd := pos + len(line)
		state.processLine(line, pos, lineEnd, i, content)
		pos = lineEnd + 1
	}

	result := filterResult(state, content)
	expected := "line1\nline2\nline3"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFilterStateStartAt(t *testing.T) {
	content := []byte("line1\nline2\nline3\nline4\n")
	startPat := compilePattern([]byte("line2"))
	r := &snippetRef{
		sourceFile: "test.go",
		startAt:    &startPat,
	}
	state := newFilterState(r)

	lines := bytes.Split(content, []byte{'\n'})
	pos := 0
	for i, line := range lines[:len(lines)-1] {
		lineEnd := pos + len(line)
		state.processLine(line, pos, lineEnd, i, content)
		pos = lineEnd + 1
	}

	result := filterResult(state, content)
	expected := "line3\nline4"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFilterStateStartOffset(t *testing.T) {
	content := []byte("line1\nline2\nline3\nline4\n")
	startPat := compilePattern([]byte("line2"))
	r := &snippetRef{
		sourceFile:  "test.go",
		startAt:     &startPat,
		startOffset: 1, // skip 1 line after match
	}
	state := newFilterState(r)

	lines := bytes.Split(content, []byte{'\n'})
	pos := 0
	for i, line := range lines[:len(lines)-1] {
		lineEnd := pos + len(line)
		state.processLine(line, pos, lineEnd, i, content)
		pos = lineEnd + 1
	}

	result := filterResult(state, content)
	expected := "line4"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFilterStateNegativeStartOffset(t *testing.T) {
	content := []byte("line1\nline2\nline3\nline4\n")
	startPat := compilePattern([]byte("line3"))
	r := &snippetRef{
		sourceFile:  "test.go",
		startAt:     &startPat,
		startOffset: -1, // include 1 line before match
	}
	state := newFilterState(r)

	lines := bytes.Split(content, []byte{'\n'})
	pos := 0
	for i, line := range lines[:len(lines)-1] {
		lineEnd := pos + len(line)
		state.processLine(line, pos, lineEnd, i, content)
		pos = lineEnd + 1
	}

	result := filterResult(state, content)
	// Should include line2 (1 before match), then line4 (line3 excluded as match line)
	expected := "line2\nline4"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFilterStateEndAt(t *testing.T) {
	content := []byte("line1\nline2\nline3\nline4\n")
	endPat := compilePattern([]byte("line3"))
	r := &snippetRef{
		sourceFile: "test.go",
		endAt:      &endPat,
		endOffset:  0,
	}
	state := newFilterState(r)

	lines := bytes.Split(content, []byte{'\n'})
	pos := 0
	for i, line := range lines[:len(lines)-1] {
		lineEnd := pos + len(line)
		state.processLine(line, pos, lineEnd, i, content)
		pos = lineEnd + 1
	}

	result := filterResult(state, content)
	expected := "line1\nline2"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFilterStateEndOffset(t *testing.T) {
	// 3 occurrences of the end pattern
	content := []byte("line1\nEND\nline3\nEND\nline5\nEND\nline7\n")
	endPat := compilePattern([]byte("END"))
	r := &snippetRef{
		sourceFile: "test.go",
		endAt:      &endPat,
		endOffset:  2, // include 2 matches of end-at, stop on 3rd
	}
	state := newFilterState(r)

	lines := bytes.Split(content, []byte{'\n'})
	pos := 0
	for i, line := range lines[:len(lines)-1] {
		lineEnd := pos + len(line)
		state.processLine(line, pos, lineEnd, i, content)
		pos = lineEnd + 1
	}

	result := filterResult(state, content)
	// Should include: line1, END(1st, include), line3, END(2nd, include), line5, then 3rd END triggers stop
	expected := "line1\nEND\nline3\nEND\nline5"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFilterStateSkipMatch(t *testing.T) {
	content := []byte("line1\n// comment\nline3\n")
	skipPat := compilePattern([]byte("^//"))
	r := &snippetRef{
		sourceFile: "test.go",
		skipMatch:  []compiledPattern{skipPat},
	}
	state := newFilterState(r)

	lines := bytes.Split(content, []byte{'\n'})
	pos := 0
	for i, line := range lines[:len(lines)-1] {
		lineEnd := pos + len(line)
		state.processLine(line, pos, lineEnd, i, content)
		pos = lineEnd + 1
	}

	result := filterResult(state, content)
	expected := "line1\nline3"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFilterStateSkipAfter(t *testing.T) {
	content := []byte("line1\nimport\nskip1\nskip2\nline5\n")
	skipPat := compilePattern([]byte("^import"))
	r := &snippetRef{
		sourceFile: "test.go",
		skipAfter: []skipAfterRule{
			{pattern: skipPat, count: 2},
		},
	}
	state := newFilterState(r)

	lines := bytes.Split(content, []byte{'\n'})
	pos := 0
	for i, line := range lines[:len(lines)-1] {
		lineEnd := pos + len(line)
		state.processLine(line, pos, lineEnd, i, content)
		pos = lineEnd + 1
	}

	result := filterResult(state, content)
	expected := "line1\nline5"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFilterStateSkipBetween(t *testing.T) {
	content := []byte("line1\nSTART\nskip1\nskip2\nEND\nline6\n")
	startPat := compilePattern([]byte("START"))
	endPat := compilePattern([]byte("END"))
	r := &snippetRef{
		sourceFile: "test.go",
		skipBetween: []skipBetweenRule{
			{start: startPat, end: endPat},
		},
	}
	state := newFilterState(r)

	lines := bytes.Split(content, []byte{'\n'})
	pos := 0
	for i, line := range lines[:len(lines)-1] {
		lineEnd := pos + len(line)
		state.processLine(line, pos, lineEnd, i, content)
		pos = lineEnd + 1
	}

	result := filterResult(state, content)
	expected := "line1\nline6"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestDedent(t *testing.T) {
	content := []byte("    line1\n    line2\n    line3\n")
	r := &snippetRef{sourceFile: "test.go"}
	state := newFilterState(r)

	lines := bytes.Split(content, []byte{'\n'})
	pos := 0
	for i, line := range lines[:len(lines)-1] {
		lineEnd := pos + len(line)
		state.processLine(line, pos, lineEnd, i, content)
		pos = lineEnd + 1
	}

	result := filterResult(state, content)
	expected := "line1\nline2\nline3"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestDedentMixed(t *testing.T) {
	content := []byte("    line1\n        line2\n    line3\n")
	r := &snippetRef{sourceFile: "test.go"}
	state := newFilterState(r)

	lines := bytes.Split(content, []byte{'\n'})
	pos := 0
	for i, line := range lines[:len(lines)-1] {
		lineEnd := pos + len(line)
		state.processLine(line, pos, lineEnd, i, content)
		pos = lineEnd + 1
	}

	result := filterResult(state, content)
	expected := "line1\n    line2\nline3"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
