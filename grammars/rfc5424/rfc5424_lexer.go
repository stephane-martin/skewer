// Code generated from grammars/rfc5424/RFC5424.g4 by ANTLR 4.7.1. DO NOT EDIT.

package rfc5424

import (
	"fmt"
	"unicode"

	"github.com/antlr/antlr4/runtime/Go/antlr"
)

// Suppress unused import error
var _ = fmt.Printf
var _ = unicode.IsLetter

var serializedLexerAtn = []uint16{
	3, 24715, 42794, 33075, 47597, 16764, 15335, 30598, 22884, 2, 18, 67, 8,
	1, 4, 2, 9, 2, 4, 3, 9, 3, 4, 4, 9, 4, 4, 5, 9, 5, 4, 6, 9, 6, 4, 7, 9,
	7, 4, 8, 9, 8, 4, 9, 9, 9, 4, 10, 9, 10, 4, 11, 9, 11, 4, 12, 9, 12, 4,
	13, 9, 13, 4, 14, 9, 14, 4, 15, 9, 15, 4, 16, 9, 16, 4, 17, 9, 17, 3, 2,
	3, 2, 3, 3, 3, 3, 3, 4, 3, 4, 3, 5, 3, 5, 3, 6, 3, 6, 3, 7, 3, 7, 3, 8,
	3, 8, 3, 9, 3, 9, 3, 10, 3, 10, 3, 11, 3, 11, 3, 12, 3, 12, 3, 13, 3, 13,
	3, 14, 3, 14, 3, 15, 3, 15, 3, 16, 3, 16, 3, 17, 3, 17, 2, 2, 18, 3, 3,
	5, 4, 7, 5, 9, 6, 11, 7, 13, 8, 15, 9, 17, 10, 19, 11, 21, 12, 23, 13,
	25, 14, 27, 15, 29, 16, 31, 17, 33, 18, 3, 2, 5, 3, 2, 50, 59, 9, 2, 35,
	35, 37, 44, 46, 46, 49, 49, 61, 61, 65, 92, 96, 128, 9, 2, 34, 34, 36,
	36, 45, 45, 47, 48, 60, 60, 62, 64, 93, 95, 2, 66, 2, 3, 3, 2, 2, 2, 2,
	5, 3, 2, 2, 2, 2, 7, 3, 2, 2, 2, 2, 9, 3, 2, 2, 2, 2, 11, 3, 2, 2, 2, 2,
	13, 3, 2, 2, 2, 2, 15, 3, 2, 2, 2, 2, 17, 3, 2, 2, 2, 2, 19, 3, 2, 2, 2,
	2, 21, 3, 2, 2, 2, 2, 23, 3, 2, 2, 2, 2, 25, 3, 2, 2, 2, 2, 27, 3, 2, 2,
	2, 2, 29, 3, 2, 2, 2, 2, 31, 3, 2, 2, 2, 2, 33, 3, 2, 2, 2, 3, 35, 3, 2,
	2, 2, 5, 37, 3, 2, 2, 2, 7, 39, 3, 2, 2, 2, 9, 41, 3, 2, 2, 2, 11, 43,
	3, 2, 2, 2, 13, 45, 3, 2, 2, 2, 15, 47, 3, 2, 2, 2, 17, 49, 3, 2, 2, 2,
	19, 51, 3, 2, 2, 2, 21, 53, 3, 2, 2, 2, 23, 55, 3, 2, 2, 2, 25, 57, 3,
	2, 2, 2, 27, 59, 3, 2, 2, 2, 29, 61, 3, 2, 2, 2, 31, 63, 3, 2, 2, 2, 33,
	65, 3, 2, 2, 2, 35, 36, 9, 2, 2, 2, 36, 4, 3, 2, 2, 2, 37, 38, 7, 48, 2,
	2, 38, 6, 3, 2, 2, 2, 39, 40, 7, 62, 2, 2, 40, 8, 3, 2, 2, 2, 41, 42, 7,
	64, 2, 2, 42, 10, 3, 2, 2, 2, 43, 44, 7, 45, 2, 2, 44, 12, 3, 2, 2, 2,
	45, 46, 7, 47, 2, 2, 46, 14, 3, 2, 2, 2, 47, 48, 7, 60, 2, 2, 48, 16, 3,
	2, 2, 2, 49, 50, 7, 34, 2, 2, 50, 18, 3, 2, 2, 2, 51, 52, 7, 63, 2, 2,
	52, 20, 3, 2, 2, 2, 53, 54, 7, 36, 2, 2, 54, 22, 3, 2, 2, 2, 55, 56, 7,
	93, 2, 2, 56, 24, 3, 2, 2, 2, 57, 58, 7, 95, 2, 2, 58, 26, 3, 2, 2, 2,
	59, 60, 7, 94, 2, 2, 60, 28, 3, 2, 2, 2, 61, 62, 9, 3, 2, 2, 62, 30, 3,
	2, 2, 2, 63, 64, 10, 4, 2, 2, 64, 32, 3, 2, 2, 2, 65, 66, 11, 2, 2, 2,
	66, 34, 3, 2, 2, 2, 3, 2, 2,
}

var lexerDeserializer = antlr.NewATNDeserializer(nil)
var lexerAtn = lexerDeserializer.DeserializeFromUInt16(serializedLexerAtn)

var lexerChannelNames = []string{
	"DEFAULT_TOKEN_CHANNEL", "HIDDEN",
}

var lexerModeNames = []string{
	"DEFAULT_MODE",
}

var lexerLiteralNames = []string{
	"", "", "'.'", "'<'", "'>'", "'+'", "'-'", "':'", "' '", "'='", "'\"'",
	"'['", "']'", "'\\'",
}

var lexerSymbolicNames = []string{
	"", "DIGIT", "DOT", "OPENBRACKET", "CLOSEBRACKET", "PLUS", "HYPHEN", "COLON",
	"SP", "EQUAL", "QUOTE", "OPENSQUARE", "CLOSESQUARE", "ANTISLASH", "ASCII",
	"VALUECHAR", "ANYCHAR",
}

var lexerRuleNames = []string{
	"DIGIT", "DOT", "OPENBRACKET", "CLOSEBRACKET", "PLUS", "HYPHEN", "COLON",
	"SP", "EQUAL", "QUOTE", "OPENSQUARE", "CLOSESQUARE", "ANTISLASH", "ASCII",
	"VALUECHAR", "ANYCHAR",
}

type RFC5424Lexer struct {
	*antlr.BaseLexer
	channelNames []string
	modeNames    []string
	// TODO: EOF string
}

var lexerDecisionToDFA = make([]*antlr.DFA, len(lexerAtn.DecisionToState))

func init() {
	for index, ds := range lexerAtn.DecisionToState {
		lexerDecisionToDFA[index] = antlr.NewDFA(ds, index)
	}
}

func NewRFC5424Lexer(input antlr.CharStream) *RFC5424Lexer {

	l := new(RFC5424Lexer)

	l.BaseLexer = antlr.NewBaseLexer(input)
	l.Interpreter = antlr.NewLexerATNSimulator(l, lexerAtn, lexerDecisionToDFA, antlr.NewPredictionContextCache())

	l.channelNames = lexerChannelNames
	l.modeNames = lexerModeNames
	l.RuleNames = lexerRuleNames
	l.LiteralNames = lexerLiteralNames
	l.SymbolicNames = lexerSymbolicNames
	l.GrammarFileName = "RFC5424.g4"
	// TODO: l.EOF = antlr.TokenEOF

	return l
}

// RFC5424Lexer tokens.
const (
	RFC5424LexerDIGIT        = 1
	RFC5424LexerDOT          = 2
	RFC5424LexerOPENBRACKET  = 3
	RFC5424LexerCLOSEBRACKET = 4
	RFC5424LexerPLUS         = 5
	RFC5424LexerHYPHEN       = 6
	RFC5424LexerCOLON        = 7
	RFC5424LexerSP           = 8
	RFC5424LexerEQUAL        = 9
	RFC5424LexerQUOTE        = 10
	RFC5424LexerOPENSQUARE   = 11
	RFC5424LexerCLOSESQUARE  = 12
	RFC5424LexerANTISLASH    = 13
	RFC5424LexerASCII        = 14
	RFC5424LexerVALUECHAR    = 15
	RFC5424LexerANYCHAR      = 16
)
