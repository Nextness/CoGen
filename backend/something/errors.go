// Package something implements the SOMETHING config language:
// Simple Orchestration Markup for Expression, Transformation,
// and Hierarchical Instruction Notation Generation.
package something

import (
	"fmt"
	"strings"
)

// SourceLocation identifies a source position in a .something file.
type SourceLocation struct {
	Line     int
	Col      int
	Filepath string
}

// SomethingError is a language error with source location and an optional fix.
type SomethingError struct {
	Message    string
	Line       int
	Col        int
	Filepath   string
	Suggestion string
}

// Error returns the receiver's diagnostic message.
func (e *SomethingError) Error() string {
	var b strings.Builder
	b.WriteString("ERROR: ")
	b.WriteString(e.Message)
	if e.Filepath != "" && e.Line > 0 {
		fmt.Fprintf(&b, "\n  in %s:%d:%d", e.Filepath, e.Line, e.Col)
	} else if e.Filepath != "" {
		fmt.Fprintf(&b, "\n  in %s", e.Filepath)
	} else if e.Line > 0 {
		fmt.Fprintf(&b, "\n  at line %d, col %d", e.Line, e.Col)
	}
	if e.Suggestion != "" {
		b.WriteString("\n  suggestion: ")
		b.WriteString(e.Suggestion)
	}
	return b.String()
}

// errAt constructs a SomethingError from token coordinates.
func errAt(msg string, tok Token, filepath string, suggestion string) *SomethingError {
	return &SomethingError{Message: msg, Line: tok.Line, Col: tok.Col, Filepath: filepath, Suggestion: suggestion}
}

// errLoc constructs a SomethingError from an optional source location.
func errLoc(msg string, loc *SourceLocation, filepath string, suggestion string) *SomethingError {
	if loc == nil {
		return &SomethingError{Message: msg, Filepath: filepath, Suggestion: suggestion}
	}
	if loc.Filepath != "" {
		filepath = loc.Filepath
	}
	return &SomethingError{Message: msg, Line: loc.Line, Col: loc.Col, Filepath: filepath, Suggestion: suggestion}
}
