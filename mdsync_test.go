package mdsync

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func mustChdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
}

func TestSingleBlock(t *testing.T) {
	dir := t.TempDir()
	mustChdir(t, dir)

	if err := os.WriteFile("test.go", []byte("package main\n\nfunc Foo() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	input := `
# Test
<!-- MDSYNC:go from:test.go -->
~~~go
old code
~~~
<!-- MDSYNC-END -->

More content
`

	expected := `
# Test
<!-- MDSYNC:go from:test.go -->
~~~go
package main

func Foo() {}
~~~
<!-- MDSYNC-END -->

More content
`

	var buf bytes.Buffer
	if err := Process(bytes.NewReader([]byte(input)), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, buf.String())
	}
}

func TestBacktickFence(t *testing.T) {
	dir := t.TempDir()
	mustChdir(t, dir)

	if err := os.WriteFile("test.go", []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	input := "# Test\n<!-- MDSYNC:go from:test.go -->\n```go\nold code\n```\n"
	expected := "# Test\n<!-- MDSYNC:go from:test.go -->\n```go\npackage main\n```\n"

	var buf bytes.Buffer
	if err := Process(bytes.NewReader([]byte(input)), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, buf.String())
	}
}

func TestMismatchedFenceNotParsed(t *testing.T) {
	dir := t.TempDir()
	mustChdir(t, dir)

	if err := os.WriteFile("test.go", []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Opening fence is ``` but the old block contains ~~~ before the real ```.
	// ~~~ must NOT be treated as the closing fence.
	input := "# Test\n<!-- MDSYNC:go from:test.go -->\n```go\nold code\n~~~\nmore old code\n```\nafter\n"
	expected := "# Test\n<!-- MDSYNC:go from:test.go -->\n```go\npackage main\n```\nafter\n"

	var buf bytes.Buffer
	if err := Process(bytes.NewReader([]byte(input)), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, buf.String())
	}
}

func TestMultipleBlocks(t *testing.T) {
	dir := t.TempDir()
	mustChdir(t, dir)

	if err := os.WriteFile("file1.go", []byte("func A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("file2.py", []byte("def b():\n    pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	input := `
# Test
<!-- MDSYNC:go from:file1.go -->
~~~go
old1
~~~
<!-- MDSYNC-END -->

<!-- MDSYNC:py from:file2.py -->
~~~python
old2
~~~
<!-- MDSYNC-END -->
`

	expected := `
# Test
<!-- MDSYNC:go from:file1.go -->
~~~go
func A() {}
~~~
<!-- MDSYNC-END -->

<!-- MDSYNC:py from:file2.py -->
~~~py
def b():
    pass
~~~
<!-- MDSYNC-END -->
`

	var buf bytes.Buffer
	if err := Process(bytes.NewReader([]byte(input)), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, buf.String())
	}
}

func TestIgnoresDirectiveEnd(t *testing.T) {
	dir := t.TempDir()
	mustChdir(t, dir)

	if err := os.WriteFile("test.go", []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	input := `
# Test
<!-- MDSYNC-END -->
Some content
<!-- MDSYNC:go from:test.go -->
~~~go
old
~~~
<!-- MDSYNC-END -->
`

	expected := `
# Test
<!-- MDSYNC-END -->
Some content
<!-- MDSYNC:go from:test.go -->
~~~go
package main
~~~
<!-- MDSYNC-END -->
`

	var buf bytes.Buffer
	if err := Process(bytes.NewReader([]byte(input)), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, buf.String())
	}
}

func TestEndToEnd(t *testing.T) {
	tmpDir := t.TempDir()

	sourceContent := []byte(`package main

// This is a comment
func main() {
	fmt.Println("Hello")
}

func other() {
	// stuff
}
`)
	if err := os.WriteFile(filepath.Join(tmpDir, "test.go"), sourceContent, 0o644); err != nil {
		t.Fatal(err)
	}

	mustChdir(t, tmpDir)

	input := `
# Test
<!-- MDSYNC:go from:test.go start-at:"func main" end-at:"^}" -->
~~~go
old content here
~~~
<!-- MDSYNC-END -->
`

	expected := `
# Test
<!-- MDSYNC:go from:test.go start-at:"func main" end-at:"^}" -->
~~~go
fmt.Println("Hello")
~~~
<!-- MDSYNC-END -->
`

	var buf bytes.Buffer
	if err := Process(bytes.NewReader([]byte(input)), &buf); err != nil {
		t.Fatal(err)
	}

	if buf.String() != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, buf.String())
	}
}

func TestStartSkipAndEndOffsetCountersAreIndependent(t *testing.T) {
	// line1 is before start-at, so it's excluded.
	// START triggers start-at; start-offset:2 skips line3 and line4.
	// END is the first end-at match; end-offset:1 includes it and stops on the second match.
	// There is no second END, so line6 and line7 are also included.
	content := []byte("line1\nSTART\nline3\nline4\nEND\nline6\nline7\n")
	startPat := compilePattern([]byte("START"))
	endPat := compilePattern([]byte("END"))
	r := &snippetRef{
		sourceFile:  "test.go",
		startAt:     &startPat,
		startOffset: 2,
		endAt:       &endPat,
		endOffset:   1,
	}
	state := newFilterState(r)

	lines := bytes.Split(content, []byte{'\n'})
	pos := 0
	for i, line := range lines[:len(lines)-1] {
		lineEnd := pos + len(line)
		state.processLine(line, pos, lineEnd, i, content)
		pos = lineEnd + 1
	}

	var buf bytes.Buffer
	state.writeTo(&buf, content)
	expected := "END\nline6\nline7"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestTabSize(t *testing.T) {
	tmpDir := t.TempDir()

	sourceContent := []byte("\tline1\n\t\tline2\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "src.go"), sourceContent, 0o644); err != nil {
		t.Fatal(err)
	}

	mustChdir(t, tmpDir)

	input := "<!-- MDSYNC from:src.go tab-size:4 -->\n```\n```\n"
	expected := "<!-- MDSYNC from:src.go tab-size:4 -->\n```\nline1\n    line2\n```\n"

	var buf bytes.Buffer
	if err := Process(bytes.NewReader([]byte(input)), &buf); err != nil {
		t.Fatal(err)
	}

	if buf.String() != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, buf.String())
	}
}

func TestTabSizeDisabled(t *testing.T) {
	tmpDir := t.TempDir()

	// Two lines with different indentation: one tab and two tabs.
	// With tab-size:-1, tabs are preserved raw; minIndent is 1 (raw byte), so both
	// lines get their leading single tab stripped; the second line retains the second tab.
	sourceContent := []byte("\tfoo\n\t\tbar\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "src.go"), sourceContent, 0o644); err != nil {
		t.Fatal(err)
	}

	mustChdir(t, tmpDir)

	input := "<!-- MDSYNC from:src.go tab-size:-1 -->\n```\n```\n"
	expected := "<!-- MDSYNC from:src.go tab-size:-1 -->\n```\nfoo\n\tbar\n```\n"

	var buf bytes.Buffer
	if err := Process(bytes.NewReader([]byte(input)), &buf); err != nil {
		t.Fatal(err)
	}

	if buf.String() != expected {
		t.Errorf("expected:\n%q\ngot:\n%q", expected, buf.String())
	}
}
