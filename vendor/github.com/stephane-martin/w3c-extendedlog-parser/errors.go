package parser

import "errors"

// ErrQuoteLeftOpen is the error returned by ExtractStrings when the input
// shows an unclosed quoted string.
var ErrQuoteLeftOpen = errors.New("Unclosed quote in a W3C log line")

// ErrEndlineInsideQuotes is the error returned when the input shows an
// endline character inside a quoted string.
var ErrEndlineInsideQuotes = errors.New("Endline character appears in a quoted string")

// ErrNoEndline is the error returned by ExtractStrings when the input
// does not end with an endline character.
var ErrNoEndline = errors.New("No endline at end of input")
