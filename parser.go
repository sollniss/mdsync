package mdsync

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
)

type snippetRef struct {
	lang        []byte
	sourceFile  string
	skipMatch   []compiledPattern
	skipAfter   []skipAfterRule
	skipBetween []skipBetweenRule
	startAt     *compiledPattern
	startOffset int
	endAt       *compiledPattern
	endOffset   int
	tabSize     int
}

type skipAfterRule struct {
	pattern compiledPattern
	count   int
}

type skipBetweenRule struct {
	start compiledPattern
	end   compiledPattern
}

type compiledPattern struct {
	literal []byte
	regex   *regexp.Regexp
}

var (
	backtickFence    = []byte("```")
	tildeFence       = []byte("~~~")
	directivePrefix  = []byte("<!-- MDSYNC")
	directiveEndMark = []byte("<!-- MDSYNC-END")
	commentClose     = []byte("-->")
)

// scanLinesKeepEndings is a bufio.SplitFunc that splits on '\n' while preserving
// the line ending in the returned token, for byte-identical passthrough output.
func scanLinesKeepEndings(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		return i + 1, data[:i+1], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// extract reads the source file referenced by ref, filters its lines according
// to the directive rules, and writes the result to w. Returns true if any
// content was written.
func extract(ref *snippetRef, w io.Writer) (bool, error) {
	content, err := os.ReadFile(ref.sourceFile)
	if err != nil {
		return false, fmt.Errorf("failed to read %s: %w", ref.sourceFile, err)
	}

	state := newFilterState(ref)

	lineNum := 0
	pos := 0
	for pos < len(content) {
		nlIdx := bytes.IndexByte(content[pos:], '\n')
		var lineEnd, nextPos int
		if nlIdx == -1 {
			lineEnd = len(content)
			nextPos = len(content)
		} else {
			lineEnd = pos + nlIdx
			nextPos = lineEnd + 1
		}

		state.processLine(content[pos:lineEnd], pos, lineEnd, lineNum, content)
		if state.ended {
			break
		}

		pos = nextPos
		lineNum++
	}

	return state.writeTo(w, content)
}

// Process streams a markdown file from r to w, replacing the content of each
// fenced code block preceded by an mdsync directive with freshly extracted
// content from the referenced source file.
func Process(r io.Reader, w io.Writer) error {
	const (
		stNormal = iota
		stMultiLineComment
		stAfterDirective
		stInCodeBlock
	)

	scanner := bufio.NewScanner(r)
	scanner.Split(scanLinesKeepEndings)
	scanner.Buffer(make([]byte, 4096), 1<<20)

	bw := bufio.NewWriter(w)

	var (
		comment     []byte
		ref         *snippetRef
		fenceMarker []byte
		state       = stNormal
	)

	for scanner.Scan() {
		line := scanner.Bytes()
		trimmed := bytes.TrimSpace(line)

		switch state {
		case stNormal:
			if bytes.HasPrefix(trimmed, directivePrefix) && !bytes.HasPrefix(trimmed, directiveEndMark) {
				if _, err := bw.Write(line); err != nil {
					return err
				}
				comment = append(comment[:0], line...)
				if bytes.HasSuffix(trimmed, commentClose) {
					var err error
					ref, err = parseRef(comment)
					if err != nil {
						return fmt.Errorf("failed to parse directive: %w", err)
					}
					if ref.sourceFile == "" {
						return fmt.Errorf("directive missing 'from:' attribute")
					}
					state = stAfterDirective
				} else {
					state = stMultiLineComment
				}
			} else {
				if _, err := bw.Write(line); err != nil {
					return err
				}
			}

		case stMultiLineComment:
			if _, err := bw.Write(line); err != nil {
				return err
			}
			comment = append(comment, line...)
			if bytes.Contains(line, commentClose) {
				var err error
				ref, err = parseRef(comment)
				if err != nil {
					return fmt.Errorf("failed to parse directive: %w", err)
				}
				if ref.sourceFile == "" {
					return fmt.Errorf("directive missing 'from:' attribute")
				}
				state = stAfterDirective
			}

		case stAfterDirective:
			if bytes.HasPrefix(line, backtickFence) {
				fenceMarker = backtickFence
			} else if bytes.HasPrefix(line, tildeFence) {
				fenceMarker = tildeFence
			} else {
				fenceMarker = nil
			}
			if fenceMarker != nil {
				if _, err := bw.Write(fenceMarker); err != nil {
					return err
				}
				if _, err := bw.Write(ref.lang); err != nil {
					return err
				}
				if err := bw.WriteByte('\n'); err != nil {
					return err
				}
				hasCode, err := extract(ref, bw)
				if err != nil {
					return err
				}
				if hasCode {
					if err := bw.WriteByte('\n'); err != nil {
						return err
					}
				}
				state = stInCodeBlock
			} else {
				if _, err := bw.Write(line); err != nil {
					return err
				}
			}

		case stInCodeBlock:
			if bytes.HasPrefix(line, fenceMarker) {
				if _, err := bw.Write(line); err != nil {
					return err
				}
				state = stNormal
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading markdown: %w", err)
	}

	return bw.Flush()
}

func parseRef(comment []byte) (*snippetRef, error) {
	_, body, ok := bytes.Cut(comment, []byte("MDSYNC"))
	if !ok {
		return nil, fmt.Errorf("invalid directive: missing MDSYNC keyword")
	}

	if idx := bytes.LastIndex(body, []byte("-->")); idx != -1 {
		body = body[:idx]
	}

	l := newLexer(body)
	r, err := parseDirective(l)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	return r, nil
}

func parseDirective(l *lexer) (*snippetRef, error) {
	r := &snippetRef{tabSize: 2}

	tok, err := l.peek()
	if err != nil {
		return nil, err
	}

	switch tok.kind {
	case tokColon:
		l.next()
		tok, err = l.next()
		if err != nil {
			return nil, err
		}
		if tok.kind == tokIdent {
			r.lang = tok.val
		}
	case tokIdent:
		nextTok, err := l.next()
		if err != nil {
			return nil, err
		}
		peek, err := l.peek()
		if err != nil {
			return nil, err
		}
		if peek.kind != tokColon {
			r.lang = nextTok.val
		} else {
			l.pos -= len(nextTok.val)
		}
	}

	for {
		tok, err := l.next()
		if err != nil {
			return nil, err
		}
		if tok.kind == tokEOF {
			break
		}

		if tok.kind != tokIdent {
			return nil, fmt.Errorf("expected attribute name at position %d", tok.pos)
		}

		key := string(tok.val)

		tok, err = l.next()
		if err != nil {
			return nil, err
		}

		if tok.kind != tokColon {
			return nil, fmt.Errorf("expected ':' after attribute %q", key)
		}

		switch key {
		case "from":
			val, err := l.expectValue()
			if err != nil {
				return nil, fmt.Errorf("attribute 'from': %w", err)
			}
			r.sourceFile = string(val)

		case "skip-match":
			val, err := l.expectString()
			if err != nil {
				return nil, fmt.Errorf("attribute 'skip-match': %w", err)
			}
			r.skipMatch = append(r.skipMatch, compilePattern(val))

		case "skip-after":
			val, err := l.expectString()
			if err != nil {
				return nil, fmt.Errorf("attribute 'skip-after': %w", err)
			}
			r.skipAfter = append(r.skipAfter, skipAfterRule{pattern: compilePattern(val)})

		case "count":
			val, err := l.expectInt()
			if err != nil {
				return nil, fmt.Errorf("attribute 'count': %w", err)
			}
			if len(r.skipAfter) == 0 {
				return nil, fmt.Errorf("'count' without preceding 'skip-after'")
			}
			r.skipAfter[len(r.skipAfter)-1].count = val

		case "skip-between":
			startVal, err := l.expectString()
			if err != nil {
				return nil, fmt.Errorf("attribute 'skip-between' start: %w", err)
			}
			tok, err := l.next()
			if err != nil {
				return nil, err
			}
			if tok.kind != tokColon {
				return nil, fmt.Errorf("expected ':' between skip-between patterns")
			}
			endVal, err := l.expectString()
			if err != nil {
				return nil, fmt.Errorf("attribute 'skip-between' end: %w", err)
			}
			r.skipBetween = append(r.skipBetween, skipBetweenRule{
				start: compilePattern(startVal),
				end:   compilePattern(endVal),
			})

		case "start-at":
			val, err := l.expectString()
			if err != nil {
				return nil, fmt.Errorf("attribute 'start-at': %w", err)
			}
			cp := compilePattern(val)
			r.startAt = &cp

		case "start-offset":
			val, err := l.expectInt()
			if err != nil {
				return nil, fmt.Errorf("attribute 'start-offset': %w", err)
			}
			r.startOffset = val

		case "end-at":
			val, err := l.expectString()
			if err != nil {
				return nil, fmt.Errorf("attribute 'end-at': %w", err)
			}
			cp := compilePattern(val)
			r.endAt = &cp

		case "end-offset":
			val, err := l.expectInt()
			if err != nil {
				return nil, fmt.Errorf("attribute 'end-offset': %w", err)
			}
			r.endOffset = val

		case "tab-size":
			val, err := l.expectInt()
			if err != nil {
				return nil, fmt.Errorf("attribute 'tab-size': %w", err)
			}
			r.tabSize = val

		default:
			return nil, fmt.Errorf("unknown attribute %q at position %d", key, tok.pos)
		}
	}

	return r, nil
}

func compilePattern(b []byte) compiledPattern {
	cp := compiledPattern{literal: b}
	re, err := regexp.Compile(string(b))
	if err == nil {
		cp.regex = re
	}
	return cp
}

func (cp *compiledPattern) matches(line []byte) bool {
	if cp.regex != nil {
		return cp.regex.Match(line)
	}
	if bytes.Contains(line, cp.literal) {
		return true
	}
	return false
}
