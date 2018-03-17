// Code generated from grammars/rfc5424/RFC5424.g4 by ANTLR 4.7.1. DO NOT EDIT.

package rfc5424 // RFC5424
import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/antlr/antlr4/runtime/Go/antlr"
)

// Suppress unused import errors
var _ = fmt.Printf
var _ = reflect.Copy
var _ = strconv.Itoa

var parserATN = []uint16{
	3, 24715, 42794, 33075, 47597, 16764, 15335, 30598, 22884, 3, 18, 225,
	4, 2, 9, 2, 4, 3, 9, 3, 4, 4, 9, 4, 4, 5, 9, 5, 4, 6, 9, 6, 4, 7, 9, 7,
	4, 8, 9, 8, 4, 9, 9, 9, 4, 10, 9, 10, 4, 11, 9, 11, 4, 12, 9, 12, 4, 13,
	9, 13, 4, 14, 9, 14, 4, 15, 9, 15, 4, 16, 9, 16, 4, 17, 9, 17, 4, 18, 9,
	18, 4, 19, 9, 19, 4, 20, 9, 20, 3, 2, 3, 2, 6, 2, 43, 10, 2, 13, 2, 14,
	2, 44, 3, 2, 3, 2, 6, 2, 49, 10, 2, 13, 2, 14, 2, 50, 3, 2, 5, 2, 54, 10,
	2, 3, 2, 3, 2, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 6, 3, 63, 10, 3, 13, 3, 14,
	3, 64, 3, 3, 3, 3, 6, 3, 69, 10, 3, 13, 3, 14, 3, 70, 3, 3, 3, 3, 6, 3,
	75, 10, 3, 13, 3, 14, 3, 76, 3, 3, 3, 3, 6, 3, 81, 10, 3, 13, 3, 14, 3,
	82, 3, 3, 3, 3, 6, 3, 87, 10, 3, 13, 3, 14, 3, 88, 3, 3, 3, 3, 3, 4, 7,
	4, 94, 10, 4, 12, 4, 14, 4, 97, 11, 4, 3, 5, 3, 5, 3, 5, 3, 5, 3, 5, 3,
	6, 3, 6, 3, 6, 3, 6, 3, 6, 3, 6, 3, 6, 3, 6, 3, 6, 3, 6, 3, 6, 3, 7, 3,
	7, 3, 7, 3, 7, 3, 7, 3, 7, 3, 7, 3, 7, 3, 7, 3, 7, 6, 7, 125, 10, 7, 13,
	7, 14, 7, 126, 5, 7, 129, 10, 7, 3, 8, 3, 8, 3, 8, 3, 8, 3, 8, 3, 8, 3,
	8, 5, 8, 138, 10, 8, 3, 9, 6, 9, 141, 10, 9, 13, 9, 14, 9, 142, 3, 10,
	6, 10, 146, 10, 10, 13, 10, 14, 10, 147, 3, 11, 6, 11, 151, 10, 11, 13,
	11, 14, 11, 152, 3, 12, 6, 12, 156, 10, 12, 13, 12, 14, 12, 157, 3, 13,
	6, 13, 161, 10, 13, 13, 13, 14, 13, 162, 3, 14, 6, 14, 166, 10, 14, 13,
	14, 14, 14, 167, 3, 15, 3, 15, 6, 15, 172, 10, 15, 13, 15, 14, 15, 173,
	5, 15, 176, 10, 15, 3, 16, 3, 16, 3, 16, 3, 16, 7, 16, 182, 10, 16, 12,
	16, 14, 16, 185, 11, 16, 3, 16, 3, 16, 3, 17, 3, 17, 3, 18, 3, 18, 3, 18,
	3, 18, 3, 18, 3, 18, 3, 19, 6, 19, 198, 10, 19, 13, 19, 14, 19, 199, 3,
	20, 3, 20, 3, 20, 3, 20, 3, 20, 3, 20, 3, 20, 3, 20, 3, 20, 3, 20, 3, 20,
	3, 20, 3, 20, 3, 20, 3, 20, 3, 20, 3, 20, 3, 20, 7, 20, 220, 10, 20, 12,
	20, 14, 20, 223, 11, 20, 3, 20, 3, 95, 2, 21, 2, 4, 6, 8, 10, 12, 14, 16,
	18, 20, 22, 24, 26, 28, 30, 32, 34, 36, 38, 2, 5, 3, 2, 7, 8, 4, 2, 3,
	9, 11, 16, 5, 2, 3, 9, 13, 13, 15, 16, 2, 242, 2, 40, 3, 2, 2, 2, 4, 57,
	3, 2, 2, 2, 6, 95, 3, 2, 2, 2, 8, 98, 3, 2, 2, 2, 10, 103, 3, 2, 2, 2,
	12, 114, 3, 2, 2, 2, 14, 137, 3, 2, 2, 2, 16, 140, 3, 2, 2, 2, 18, 145,
	3, 2, 2, 2, 20, 150, 3, 2, 2, 2, 22, 155, 3, 2, 2, 2, 24, 160, 3, 2, 2,
	2, 26, 165, 3, 2, 2, 2, 28, 175, 3, 2, 2, 2, 30, 177, 3, 2, 2, 2, 32, 188,
	3, 2, 2, 2, 34, 190, 3, 2, 2, 2, 36, 197, 3, 2, 2, 2, 38, 221, 3, 2, 2,
	2, 40, 42, 5, 4, 3, 2, 41, 43, 7, 10, 2, 2, 42, 41, 3, 2, 2, 2, 43, 44,
	3, 2, 2, 2, 44, 42, 3, 2, 2, 2, 44, 45, 3, 2, 2, 2, 45, 46, 3, 2, 2, 2,
	46, 53, 5, 28, 15, 2, 47, 49, 7, 10, 2, 2, 48, 47, 3, 2, 2, 2, 49, 50,
	3, 2, 2, 2, 50, 48, 3, 2, 2, 2, 50, 51, 3, 2, 2, 2, 51, 52, 3, 2, 2, 2,
	52, 54, 5, 6, 4, 2, 53, 48, 3, 2, 2, 2, 53, 54, 3, 2, 2, 2, 54, 55, 3,
	2, 2, 2, 55, 56, 7, 2, 2, 3, 56, 3, 3, 2, 2, 2, 57, 58, 7, 5, 2, 2, 58,
	59, 5, 16, 9, 2, 59, 60, 7, 6, 2, 2, 60, 62, 5, 18, 10, 2, 61, 63, 7, 10,
	2, 2, 62, 61, 3, 2, 2, 2, 63, 64, 3, 2, 2, 2, 64, 62, 3, 2, 2, 2, 64, 65,
	3, 2, 2, 2, 65, 66, 3, 2, 2, 2, 66, 68, 5, 8, 5, 2, 67, 69, 7, 10, 2, 2,
	68, 67, 3, 2, 2, 2, 69, 70, 3, 2, 2, 2, 70, 68, 3, 2, 2, 2, 70, 71, 3,
	2, 2, 2, 71, 72, 3, 2, 2, 2, 72, 74, 5, 20, 11, 2, 73, 75, 7, 10, 2, 2,
	74, 73, 3, 2, 2, 2, 75, 76, 3, 2, 2, 2, 76, 74, 3, 2, 2, 2, 76, 77, 3,
	2, 2, 2, 77, 78, 3, 2, 2, 2, 78, 80, 5, 22, 12, 2, 79, 81, 7, 10, 2, 2,
	80, 79, 3, 2, 2, 2, 81, 82, 3, 2, 2, 2, 82, 80, 3, 2, 2, 2, 82, 83, 3,
	2, 2, 2, 83, 84, 3, 2, 2, 2, 84, 86, 5, 26, 14, 2, 85, 87, 7, 10, 2, 2,
	86, 85, 3, 2, 2, 2, 87, 88, 3, 2, 2, 2, 88, 86, 3, 2, 2, 2, 88, 89, 3,
	2, 2, 2, 89, 90, 3, 2, 2, 2, 90, 91, 5, 24, 13, 2, 91, 5, 3, 2, 2, 2, 92,
	94, 11, 2, 2, 2, 93, 92, 3, 2, 2, 2, 94, 97, 3, 2, 2, 2, 95, 96, 3, 2,
	2, 2, 95, 93, 3, 2, 2, 2, 96, 7, 3, 2, 2, 2, 97, 95, 3, 2, 2, 2, 98, 99,
	5, 10, 6, 2, 99, 100, 7, 16, 2, 2, 100, 101, 5, 12, 7, 2, 101, 102, 5,
	14, 8, 2, 102, 9, 3, 2, 2, 2, 103, 104, 7, 3, 2, 2, 104, 105, 7, 3, 2,
	2, 105, 106, 7, 3, 2, 2, 106, 107, 7, 3, 2, 2, 107, 108, 7, 8, 2, 2, 108,
	109, 7, 3, 2, 2, 109, 110, 7, 3, 2, 2, 110, 111, 7, 8, 2, 2, 111, 112,
	7, 3, 2, 2, 112, 113, 7, 3, 2, 2, 113, 11, 3, 2, 2, 2, 114, 115, 7, 3,
	2, 2, 115, 116, 7, 3, 2, 2, 116, 117, 7, 9, 2, 2, 117, 118, 7, 3, 2, 2,
	118, 119, 7, 3, 2, 2, 119, 120, 7, 9, 2, 2, 120, 121, 7, 3, 2, 2, 121,
	128, 7, 3, 2, 2, 122, 124, 7, 4, 2, 2, 123, 125, 7, 3, 2, 2, 124, 123,
	3, 2, 2, 2, 125, 126, 3, 2, 2, 2, 126, 124, 3, 2, 2, 2, 126, 127, 3, 2,
	2, 2, 127, 129, 3, 2, 2, 2, 128, 122, 3, 2, 2, 2, 128, 129, 3, 2, 2, 2,
	129, 13, 3, 2, 2, 2, 130, 138, 7, 16, 2, 2, 131, 132, 9, 2, 2, 2, 132,
	133, 7, 3, 2, 2, 133, 134, 7, 3, 2, 2, 134, 135, 7, 9, 2, 2, 135, 136,
	7, 3, 2, 2, 136, 138, 7, 3, 2, 2, 137, 130, 3, 2, 2, 2, 137, 131, 3, 2,
	2, 2, 138, 15, 3, 2, 2, 2, 139, 141, 7, 3, 2, 2, 140, 139, 3, 2, 2, 2,
	141, 142, 3, 2, 2, 2, 142, 140, 3, 2, 2, 2, 142, 143, 3, 2, 2, 2, 143,
	17, 3, 2, 2, 2, 144, 146, 7, 3, 2, 2, 145, 144, 3, 2, 2, 2, 146, 147, 3,
	2, 2, 2, 147, 145, 3, 2, 2, 2, 147, 148, 3, 2, 2, 2, 148, 19, 3, 2, 2,
	2, 149, 151, 9, 3, 2, 2, 150, 149, 3, 2, 2, 2, 151, 152, 3, 2, 2, 2, 152,
	150, 3, 2, 2, 2, 152, 153, 3, 2, 2, 2, 153, 21, 3, 2, 2, 2, 154, 156, 9,
	3, 2, 2, 155, 154, 3, 2, 2, 2, 156, 157, 3, 2, 2, 2, 157, 155, 3, 2, 2,
	2, 157, 158, 3, 2, 2, 2, 158, 23, 3, 2, 2, 2, 159, 161, 9, 3, 2, 2, 160,
	159, 3, 2, 2, 2, 161, 162, 3, 2, 2, 2, 162, 160, 3, 2, 2, 2, 162, 163,
	3, 2, 2, 2, 163, 25, 3, 2, 2, 2, 164, 166, 9, 3, 2, 2, 165, 164, 3, 2,
	2, 2, 166, 167, 3, 2, 2, 2, 167, 165, 3, 2, 2, 2, 167, 168, 3, 2, 2, 2,
	168, 27, 3, 2, 2, 2, 169, 176, 7, 8, 2, 2, 170, 172, 5, 30, 16, 2, 171,
	170, 3, 2, 2, 2, 172, 173, 3, 2, 2, 2, 173, 171, 3, 2, 2, 2, 173, 174,
	3, 2, 2, 2, 174, 176, 3, 2, 2, 2, 175, 169, 3, 2, 2, 2, 175, 171, 3, 2,
	2, 2, 176, 29, 3, 2, 2, 2, 177, 178, 7, 13, 2, 2, 178, 183, 5, 32, 17,
	2, 179, 180, 7, 10, 2, 2, 180, 182, 5, 34, 18, 2, 181, 179, 3, 2, 2, 2,
	182, 185, 3, 2, 2, 2, 183, 181, 3, 2, 2, 2, 183, 184, 3, 2, 2, 2, 184,
	186, 3, 2, 2, 2, 185, 183, 3, 2, 2, 2, 186, 187, 7, 14, 2, 2, 187, 31,
	3, 2, 2, 2, 188, 189, 5, 36, 19, 2, 189, 33, 3, 2, 2, 2, 190, 191, 5, 36,
	19, 2, 191, 192, 7, 11, 2, 2, 192, 193, 7, 12, 2, 2, 193, 194, 5, 38, 20,
	2, 194, 195, 7, 12, 2, 2, 195, 35, 3, 2, 2, 2, 196, 198, 9, 4, 2, 2, 197,
	196, 3, 2, 2, 2, 198, 199, 3, 2, 2, 2, 199, 197, 3, 2, 2, 2, 199, 200,
	3, 2, 2, 2, 200, 37, 3, 2, 2, 2, 201, 220, 7, 16, 2, 2, 202, 220, 7, 17,
	2, 2, 203, 220, 7, 3, 2, 2, 204, 220, 7, 4, 2, 2, 205, 220, 7, 5, 2, 2,
	206, 220, 7, 6, 2, 2, 207, 220, 7, 7, 2, 2, 208, 220, 7, 8, 2, 2, 209,
	220, 7, 9, 2, 2, 210, 220, 7, 10, 2, 2, 211, 220, 7, 11, 2, 2, 212, 220,
	7, 13, 2, 2, 213, 214, 7, 15, 2, 2, 214, 220, 7, 15, 2, 2, 215, 216, 7,
	15, 2, 2, 216, 220, 7, 14, 2, 2, 217, 218, 7, 15, 2, 2, 218, 220, 7, 12,
	2, 2, 219, 201, 3, 2, 2, 2, 219, 202, 3, 2, 2, 2, 219, 203, 3, 2, 2, 2,
	219, 204, 3, 2, 2, 2, 219, 205, 3, 2, 2, 2, 219, 206, 3, 2, 2, 2, 219,
	207, 3, 2, 2, 2, 219, 208, 3, 2, 2, 2, 219, 209, 3, 2, 2, 2, 219, 210,
	3, 2, 2, 2, 219, 211, 3, 2, 2, 2, 219, 212, 3, 2, 2, 2, 219, 213, 3, 2,
	2, 2, 219, 215, 3, 2, 2, 2, 219, 217, 3, 2, 2, 2, 220, 223, 3, 2, 2, 2,
	221, 219, 3, 2, 2, 2, 221, 222, 3, 2, 2, 2, 222, 39, 3, 2, 2, 2, 223, 221,
	3, 2, 2, 2, 26, 44, 50, 53, 64, 70, 76, 82, 88, 95, 126, 128, 137, 142,
	147, 152, 157, 162, 167, 173, 175, 183, 199, 219, 221,
}
var deserializer = antlr.NewATNDeserializer(nil)
var deserializedATN = deserializer.DeserializeFromUInt16(parserATN)

var literalNames = []string{
	"", "", "'.'", "'<'", "'>'", "'+'", "'-'", "':'", "' '", "'='", "'\"'",
	"'['", "']'", "'\\'",
}
var symbolicNames = []string{
	"", "DIGIT", "DOT", "OPENBRACKET", "CLOSEBRACKET", "PLUS", "HYPHEN", "COLON",
	"SP", "EQUAL", "QUOTE", "OPENSQUARE", "CLOSESQUARE", "ANTISLASH", "ASCII",
	"VALUECHAR", "ANYCHAR",
}

var ruleNames = []string{
	"full", "headr", "msg", "timestamp", "date", "time", "timezone", "pri",
	"version", "hostname", "appname", "msgid", "procid", "structured", "element",
	"sid", "param", "name", "value",
}
var decisionToDFA = make([]*antlr.DFA, len(deserializedATN.DecisionToState))

func init() {
	for index, ds := range deserializedATN.DecisionToState {
		decisionToDFA[index] = antlr.NewDFA(ds, index)
	}
}

type RFC5424Parser struct {
	*antlr.BaseParser
}

func NewRFC5424Parser(input antlr.TokenStream) *RFC5424Parser {
	this := new(RFC5424Parser)

	this.BaseParser = antlr.NewBaseParser(input)

	this.Interpreter = antlr.NewParserATNSimulator(this, deserializedATN, decisionToDFA, antlr.NewPredictionContextCache())
	this.RuleNames = ruleNames
	this.LiteralNames = literalNames
	this.SymbolicNames = symbolicNames
	this.GrammarFileName = "RFC5424.g4"

	return this
}

// RFC5424Parser tokens.
const (
	RFC5424ParserEOF          = antlr.TokenEOF
	RFC5424ParserDIGIT        = 1
	RFC5424ParserDOT          = 2
	RFC5424ParserOPENBRACKET  = 3
	RFC5424ParserCLOSEBRACKET = 4
	RFC5424ParserPLUS         = 5
	RFC5424ParserHYPHEN       = 6
	RFC5424ParserCOLON        = 7
	RFC5424ParserSP           = 8
	RFC5424ParserEQUAL        = 9
	RFC5424ParserQUOTE        = 10
	RFC5424ParserOPENSQUARE   = 11
	RFC5424ParserCLOSESQUARE  = 12
	RFC5424ParserANTISLASH    = 13
	RFC5424ParserASCII        = 14
	RFC5424ParserVALUECHAR    = 15
	RFC5424ParserANYCHAR      = 16
)

// RFC5424Parser rules.
const (
	RFC5424ParserRULE_full       = 0
	RFC5424ParserRULE_headr      = 1
	RFC5424ParserRULE_msg        = 2
	RFC5424ParserRULE_timestamp  = 3
	RFC5424ParserRULE_date       = 4
	RFC5424ParserRULE_time       = 5
	RFC5424ParserRULE_timezone   = 6
	RFC5424ParserRULE_pri        = 7
	RFC5424ParserRULE_version    = 8
	RFC5424ParserRULE_hostname   = 9
	RFC5424ParserRULE_appname    = 10
	RFC5424ParserRULE_msgid      = 11
	RFC5424ParserRULE_procid     = 12
	RFC5424ParserRULE_structured = 13
	RFC5424ParserRULE_element    = 14
	RFC5424ParserRULE_sid        = 15
	RFC5424ParserRULE_param      = 16
	RFC5424ParserRULE_name       = 17
	RFC5424ParserRULE_value      = 18
)

// IFullContext is an interface to support dynamic dispatch.
type IFullContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsFullContext differentiates from other interfaces.
	IsFullContext()
}

type FullContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyFullContext() *FullContext {
	var p = new(FullContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_full
	return p
}

func (*FullContext) IsFullContext() {}

func NewFullContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FullContext {
	var p = new(FullContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_full

	return p
}

func (s *FullContext) GetParser() antlr.Parser { return s.parser }

func (s *FullContext) Headr() IHeadrContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IHeadrContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IHeadrContext)
}

func (s *FullContext) Structured() IStructuredContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IStructuredContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IStructuredContext)
}

func (s *FullContext) EOF() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserEOF, 0)
}

func (s *FullContext) AllSP() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserSP)
}

func (s *FullContext) SP(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserSP, i)
}

func (s *FullContext) Msg() IMsgContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IMsgContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IMsgContext)
}

func (s *FullContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FullContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *FullContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterFull(s)
	}
}

func (s *FullContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitFull(s)
	}
}

func (p *RFC5424Parser) Full() (localctx IFullContext) {
	localctx = NewFullContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 0, RFC5424ParserRULE_full)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	var _alt int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(38)
		p.Headr()
	}
	p.SetState(40)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = _la == RFC5424ParserSP {
		{
			p.SetState(39)
			p.Match(RFC5424ParserSP)
		}

		p.SetState(42)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(44)
		p.Structured()
	}
	p.SetState(51)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	if _la == RFC5424ParserSP {
		p.SetState(46)
		p.GetErrorHandler().Sync(p)
		_alt = 1
		for ok := true; ok; ok = _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
			switch _alt {
			case 1:
				{
					p.SetState(45)
					p.Match(RFC5424ParserSP)
				}

			default:
				panic(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
			}

			p.SetState(48)
			p.GetErrorHandler().Sync(p)
			_alt = p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 1, p.GetParserRuleContext())
		}
		{
			p.SetState(50)
			p.Msg()
		}

	}
	{
		p.SetState(53)
		p.Match(RFC5424ParserEOF)
	}

	return localctx
}

// IHeadrContext is an interface to support dynamic dispatch.
type IHeadrContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsHeadrContext differentiates from other interfaces.
	IsHeadrContext()
}

type HeadrContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyHeadrContext() *HeadrContext {
	var p = new(HeadrContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_headr
	return p
}

func (*HeadrContext) IsHeadrContext() {}

func NewHeadrContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *HeadrContext {
	var p = new(HeadrContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_headr

	return p
}

func (s *HeadrContext) GetParser() antlr.Parser { return s.parser }

func (s *HeadrContext) OPENBRACKET() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserOPENBRACKET, 0)
}

func (s *HeadrContext) Pri() IPriContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IPriContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IPriContext)
}

func (s *HeadrContext) CLOSEBRACKET() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCLOSEBRACKET, 0)
}

func (s *HeadrContext) Version() IVersionContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IVersionContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IVersionContext)
}

func (s *HeadrContext) Timestamp() ITimestampContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*ITimestampContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(ITimestampContext)
}

func (s *HeadrContext) Hostname() IHostnameContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IHostnameContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IHostnameContext)
}

func (s *HeadrContext) Appname() IAppnameContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IAppnameContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IAppnameContext)
}

func (s *HeadrContext) Procid() IProcidContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IProcidContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IProcidContext)
}

func (s *HeadrContext) Msgid() IMsgidContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IMsgidContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IMsgidContext)
}

func (s *HeadrContext) AllSP() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserSP)
}

func (s *HeadrContext) SP(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserSP, i)
}

func (s *HeadrContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *HeadrContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *HeadrContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterHeadr(s)
	}
}

func (s *HeadrContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitHeadr(s)
	}
}

func (p *RFC5424Parser) Headr() (localctx IHeadrContext) {
	localctx = NewHeadrContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 2, RFC5424ParserRULE_headr)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(55)
		p.Match(RFC5424ParserOPENBRACKET)
	}
	{
		p.SetState(56)
		p.Pri()
	}
	{
		p.SetState(57)
		p.Match(RFC5424ParserCLOSEBRACKET)
	}
	{
		p.SetState(58)
		p.Version()
	}
	p.SetState(60)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = _la == RFC5424ParserSP {
		{
			p.SetState(59)
			p.Match(RFC5424ParserSP)
		}

		p.SetState(62)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(64)
		p.Timestamp()
	}
	p.SetState(66)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = _la == RFC5424ParserSP {
		{
			p.SetState(65)
			p.Match(RFC5424ParserSP)
		}

		p.SetState(68)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(70)
		p.Hostname()
	}
	p.SetState(72)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = _la == RFC5424ParserSP {
		{
			p.SetState(71)
			p.Match(RFC5424ParserSP)
		}

		p.SetState(74)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(76)
		p.Appname()
	}
	p.SetState(78)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = _la == RFC5424ParserSP {
		{
			p.SetState(77)
			p.Match(RFC5424ParserSP)
		}

		p.SetState(80)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(82)
		p.Procid()
	}
	p.SetState(84)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = _la == RFC5424ParserSP {
		{
			p.SetState(83)
			p.Match(RFC5424ParserSP)
		}

		p.SetState(86)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(88)
		p.Msgid()
	}

	return localctx
}

// IMsgContext is an interface to support dynamic dispatch.
type IMsgContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsMsgContext differentiates from other interfaces.
	IsMsgContext()
}

type MsgContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyMsgContext() *MsgContext {
	var p = new(MsgContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_msg
	return p
}

func (*MsgContext) IsMsgContext() {}

func NewMsgContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *MsgContext {
	var p = new(MsgContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_msg

	return p
}

func (s *MsgContext) GetParser() antlr.Parser { return s.parser }
func (s *MsgContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *MsgContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *MsgContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterMsg(s)
	}
}

func (s *MsgContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitMsg(s)
	}
}

func (p *RFC5424Parser) Msg() (localctx IMsgContext) {
	localctx = NewMsgContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 4, RFC5424ParserRULE_msg)

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	var _alt int

	p.EnterOuterAlt(localctx, 1)
	p.SetState(93)
	p.GetErrorHandler().Sync(p)
	_alt = p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 8, p.GetParserRuleContext())

	for _alt != 1 && _alt != antlr.ATNInvalidAltNumber {
		if _alt == 1+1 {
			p.SetState(90)
			p.MatchWildcard()

		}
		p.SetState(95)
		p.GetErrorHandler().Sync(p)
		_alt = p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 8, p.GetParserRuleContext())
	}

	return localctx
}

// ITimestampContext is an interface to support dynamic dispatch.
type ITimestampContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsTimestampContext differentiates from other interfaces.
	IsTimestampContext()
}

type TimestampContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyTimestampContext() *TimestampContext {
	var p = new(TimestampContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_timestamp
	return p
}

func (*TimestampContext) IsTimestampContext() {}

func NewTimestampContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *TimestampContext {
	var p = new(TimestampContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_timestamp

	return p
}

func (s *TimestampContext) GetParser() antlr.Parser { return s.parser }

func (s *TimestampContext) Date() IDateContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IDateContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IDateContext)
}

func (s *TimestampContext) ASCII() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserASCII, 0)
}

func (s *TimestampContext) Time() ITimeContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*ITimeContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(ITimeContext)
}

func (s *TimestampContext) Timezone() ITimezoneContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*ITimezoneContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(ITimezoneContext)
}

func (s *TimestampContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *TimestampContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *TimestampContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterTimestamp(s)
	}
}

func (s *TimestampContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitTimestamp(s)
	}
}

func (p *RFC5424Parser) Timestamp() (localctx ITimestampContext) {
	localctx = NewTimestampContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 6, RFC5424ParserRULE_timestamp)

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(96)
		p.Date()
	}
	{
		p.SetState(97)
		p.Match(RFC5424ParserASCII)
	}
	{
		p.SetState(98)
		p.Time()
	}
	{
		p.SetState(99)
		p.Timezone()
	}

	return localctx
}

// IDateContext is an interface to support dynamic dispatch.
type IDateContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsDateContext differentiates from other interfaces.
	IsDateContext()
}

type DateContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyDateContext() *DateContext {
	var p = new(DateContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_date
	return p
}

func (*DateContext) IsDateContext() {}

func NewDateContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *DateContext {
	var p = new(DateContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_date

	return p
}

func (s *DateContext) GetParser() antlr.Parser { return s.parser }

func (s *DateContext) AllDIGIT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDIGIT)
}

func (s *DateContext) DIGIT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, i)
}

func (s *DateContext) AllHYPHEN() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserHYPHEN)
}

func (s *DateContext) HYPHEN(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserHYPHEN, i)
}

func (s *DateContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *DateContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *DateContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterDate(s)
	}
}

func (s *DateContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitDate(s)
	}
}

func (p *RFC5424Parser) Date() (localctx IDateContext) {
	localctx = NewDateContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 8, RFC5424ParserRULE_date)

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(101)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(102)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(103)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(104)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(105)
		p.Match(RFC5424ParserHYPHEN)
	}
	{
		p.SetState(106)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(107)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(108)
		p.Match(RFC5424ParserHYPHEN)
	}
	{
		p.SetState(109)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(110)
		p.Match(RFC5424ParserDIGIT)
	}

	return localctx
}

// ITimeContext is an interface to support dynamic dispatch.
type ITimeContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsTimeContext differentiates from other interfaces.
	IsTimeContext()
}

type TimeContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyTimeContext() *TimeContext {
	var p = new(TimeContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_time
	return p
}

func (*TimeContext) IsTimeContext() {}

func NewTimeContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *TimeContext {
	var p = new(TimeContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_time

	return p
}

func (s *TimeContext) GetParser() antlr.Parser { return s.parser }

func (s *TimeContext) AllDIGIT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDIGIT)
}

func (s *TimeContext) DIGIT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, i)
}

func (s *TimeContext) AllCOLON() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserCOLON)
}

func (s *TimeContext) COLON(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCOLON, i)
}

func (s *TimeContext) DOT() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDOT, 0)
}

func (s *TimeContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *TimeContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *TimeContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterTime(s)
	}
}

func (s *TimeContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitTime(s)
	}
}

func (p *RFC5424Parser) Time() (localctx ITimeContext) {
	localctx = NewTimeContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 10, RFC5424ParserRULE_time)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(112)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(113)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(114)
		p.Match(RFC5424ParserCOLON)
	}
	{
		p.SetState(115)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(116)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(117)
		p.Match(RFC5424ParserCOLON)
	}
	{
		p.SetState(118)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(119)
		p.Match(RFC5424ParserDIGIT)
	}
	p.SetState(126)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	if _la == RFC5424ParserDOT {
		{
			p.SetState(120)
			p.Match(RFC5424ParserDOT)
		}
		p.SetState(122)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)

		for ok := true; ok; ok = _la == RFC5424ParserDIGIT {
			{
				p.SetState(121)
				p.Match(RFC5424ParserDIGIT)
			}

			p.SetState(124)
			p.GetErrorHandler().Sync(p)
			_la = p.GetTokenStream().LA(1)
		}

	}

	return localctx
}

// ITimezoneContext is an interface to support dynamic dispatch.
type ITimezoneContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsTimezoneContext differentiates from other interfaces.
	IsTimezoneContext()
}

type TimezoneContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyTimezoneContext() *TimezoneContext {
	var p = new(TimezoneContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_timezone
	return p
}

func (*TimezoneContext) IsTimezoneContext() {}

func NewTimezoneContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *TimezoneContext {
	var p = new(TimezoneContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_timezone

	return p
}

func (s *TimezoneContext) GetParser() antlr.Parser { return s.parser }

func (s *TimezoneContext) ASCII() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserASCII, 0)
}

func (s *TimezoneContext) AllDIGIT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDIGIT)
}

func (s *TimezoneContext) DIGIT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, i)
}

func (s *TimezoneContext) COLON() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCOLON, 0)
}

func (s *TimezoneContext) PLUS() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserPLUS, 0)
}

func (s *TimezoneContext) HYPHEN() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserHYPHEN, 0)
}

func (s *TimezoneContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *TimezoneContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *TimezoneContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterTimezone(s)
	}
}

func (s *TimezoneContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitTimezone(s)
	}
}

func (p *RFC5424Parser) Timezone() (localctx ITimezoneContext) {
	localctx = NewTimezoneContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 12, RFC5424ParserRULE_timezone)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.SetState(135)
	p.GetErrorHandler().Sync(p)

	switch p.GetTokenStream().LA(1) {
	case RFC5424ParserASCII:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(128)
			p.Match(RFC5424ParserASCII)
		}

	case RFC5424ParserPLUS, RFC5424ParserHYPHEN:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(129)
			_la = p.GetTokenStream().LA(1)

			if !(_la == RFC5424ParserPLUS || _la == RFC5424ParserHYPHEN) {
				p.GetErrorHandler().RecoverInline(p)
			} else {
				p.GetErrorHandler().ReportMatch(p)
				p.Consume()
			}
		}
		{
			p.SetState(130)
			p.Match(RFC5424ParserDIGIT)
		}
		{
			p.SetState(131)
			p.Match(RFC5424ParserDIGIT)
		}
		{
			p.SetState(132)
			p.Match(RFC5424ParserCOLON)
		}
		{
			p.SetState(133)
			p.Match(RFC5424ParserDIGIT)
		}
		{
			p.SetState(134)
			p.Match(RFC5424ParserDIGIT)
		}

	default:
		panic(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
	}

	return localctx
}

// IPriContext is an interface to support dynamic dispatch.
type IPriContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsPriContext differentiates from other interfaces.
	IsPriContext()
}

type PriContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyPriContext() *PriContext {
	var p = new(PriContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_pri
	return p
}

func (*PriContext) IsPriContext() {}

func NewPriContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *PriContext {
	var p = new(PriContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_pri

	return p
}

func (s *PriContext) GetParser() antlr.Parser { return s.parser }

func (s *PriContext) AllDIGIT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDIGIT)
}

func (s *PriContext) DIGIT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, i)
}

func (s *PriContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *PriContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *PriContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterPri(s)
	}
}

func (s *PriContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitPri(s)
	}
}

func (p *RFC5424Parser) Pri() (localctx IPriContext) {
	localctx = NewPriContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 14, RFC5424ParserRULE_pri)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(138)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = _la == RFC5424ParserDIGIT {
		{
			p.SetState(137)
			p.Match(RFC5424ParserDIGIT)
		}

		p.SetState(140)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}

	return localctx
}

// IVersionContext is an interface to support dynamic dispatch.
type IVersionContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsVersionContext differentiates from other interfaces.
	IsVersionContext()
}

type VersionContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyVersionContext() *VersionContext {
	var p = new(VersionContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_version
	return p
}

func (*VersionContext) IsVersionContext() {}

func NewVersionContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *VersionContext {
	var p = new(VersionContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_version

	return p
}

func (s *VersionContext) GetParser() antlr.Parser { return s.parser }

func (s *VersionContext) AllDIGIT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDIGIT)
}

func (s *VersionContext) DIGIT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, i)
}

func (s *VersionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *VersionContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *VersionContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterVersion(s)
	}
}

func (s *VersionContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitVersion(s)
	}
}

func (p *RFC5424Parser) Version() (localctx IVersionContext) {
	localctx = NewVersionContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 16, RFC5424ParserRULE_version)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(143)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = _la == RFC5424ParserDIGIT {
		{
			p.SetState(142)
			p.Match(RFC5424ParserDIGIT)
		}

		p.SetState(145)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}

	return localctx
}

// IHostnameContext is an interface to support dynamic dispatch.
type IHostnameContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsHostnameContext differentiates from other interfaces.
	IsHostnameContext()
}

type HostnameContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyHostnameContext() *HostnameContext {
	var p = new(HostnameContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_hostname
	return p
}

func (*HostnameContext) IsHostnameContext() {}

func NewHostnameContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *HostnameContext {
	var p = new(HostnameContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_hostname

	return p
}

func (s *HostnameContext) GetParser() antlr.Parser { return s.parser }

func (s *HostnameContext) AllASCII() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserASCII)
}

func (s *HostnameContext) ASCII(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserASCII, i)
}

func (s *HostnameContext) AllDIGIT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDIGIT)
}

func (s *HostnameContext) DIGIT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, i)
}

func (s *HostnameContext) AllDOT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDOT)
}

func (s *HostnameContext) DOT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDOT, i)
}

func (s *HostnameContext) AllOPENBRACKET() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserOPENBRACKET)
}

func (s *HostnameContext) OPENBRACKET(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserOPENBRACKET, i)
}

func (s *HostnameContext) AllCLOSEBRACKET() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserCLOSEBRACKET)
}

func (s *HostnameContext) CLOSEBRACKET(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCLOSEBRACKET, i)
}

func (s *HostnameContext) AllPLUS() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserPLUS)
}

func (s *HostnameContext) PLUS(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserPLUS, i)
}

func (s *HostnameContext) AllHYPHEN() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserHYPHEN)
}

func (s *HostnameContext) HYPHEN(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserHYPHEN, i)
}

func (s *HostnameContext) AllCOLON() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserCOLON)
}

func (s *HostnameContext) COLON(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCOLON, i)
}

func (s *HostnameContext) AllOPENSQUARE() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserOPENSQUARE)
}

func (s *HostnameContext) OPENSQUARE(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserOPENSQUARE, i)
}

func (s *HostnameContext) AllANTISLASH() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserANTISLASH)
}

func (s *HostnameContext) ANTISLASH(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserANTISLASH, i)
}

func (s *HostnameContext) AllEQUAL() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserEQUAL)
}

func (s *HostnameContext) EQUAL(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserEQUAL, i)
}

func (s *HostnameContext) AllCLOSESQUARE() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserCLOSESQUARE)
}

func (s *HostnameContext) CLOSESQUARE(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCLOSESQUARE, i)
}

func (s *HostnameContext) AllQUOTE() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserQUOTE)
}

func (s *HostnameContext) QUOTE(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserQUOTE, i)
}

func (s *HostnameContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *HostnameContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *HostnameContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterHostname(s)
	}
}

func (s *HostnameContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitHostname(s)
	}
}

func (p *RFC5424Parser) Hostname() (localctx IHostnameContext) {
	localctx = NewHostnameContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 18, RFC5424ParserRULE_hostname)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(148)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = (((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<RFC5424ParserDIGIT)|(1<<RFC5424ParserDOT)|(1<<RFC5424ParserOPENBRACKET)|(1<<RFC5424ParserCLOSEBRACKET)|(1<<RFC5424ParserPLUS)|(1<<RFC5424ParserHYPHEN)|(1<<RFC5424ParserCOLON)|(1<<RFC5424ParserEQUAL)|(1<<RFC5424ParserQUOTE)|(1<<RFC5424ParserOPENSQUARE)|(1<<RFC5424ParserCLOSESQUARE)|(1<<RFC5424ParserANTISLASH)|(1<<RFC5424ParserASCII))) != 0) {
		{
			p.SetState(147)
			_la = p.GetTokenStream().LA(1)

			if !(((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<RFC5424ParserDIGIT)|(1<<RFC5424ParserDOT)|(1<<RFC5424ParserOPENBRACKET)|(1<<RFC5424ParserCLOSEBRACKET)|(1<<RFC5424ParserPLUS)|(1<<RFC5424ParserHYPHEN)|(1<<RFC5424ParserCOLON)|(1<<RFC5424ParserEQUAL)|(1<<RFC5424ParserQUOTE)|(1<<RFC5424ParserOPENSQUARE)|(1<<RFC5424ParserCLOSESQUARE)|(1<<RFC5424ParserANTISLASH)|(1<<RFC5424ParserASCII))) != 0) {
				p.GetErrorHandler().RecoverInline(p)
			} else {
				p.GetErrorHandler().ReportMatch(p)
				p.Consume()
			}
		}

		p.SetState(150)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}

	return localctx
}

// IAppnameContext is an interface to support dynamic dispatch.
type IAppnameContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsAppnameContext differentiates from other interfaces.
	IsAppnameContext()
}

type AppnameContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyAppnameContext() *AppnameContext {
	var p = new(AppnameContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_appname
	return p
}

func (*AppnameContext) IsAppnameContext() {}

func NewAppnameContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *AppnameContext {
	var p = new(AppnameContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_appname

	return p
}

func (s *AppnameContext) GetParser() antlr.Parser { return s.parser }

func (s *AppnameContext) AllASCII() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserASCII)
}

func (s *AppnameContext) ASCII(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserASCII, i)
}

func (s *AppnameContext) AllDIGIT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDIGIT)
}

func (s *AppnameContext) DIGIT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, i)
}

func (s *AppnameContext) AllDOT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDOT)
}

func (s *AppnameContext) DOT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDOT, i)
}

func (s *AppnameContext) AllOPENBRACKET() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserOPENBRACKET)
}

func (s *AppnameContext) OPENBRACKET(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserOPENBRACKET, i)
}

func (s *AppnameContext) AllCLOSEBRACKET() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserCLOSEBRACKET)
}

func (s *AppnameContext) CLOSEBRACKET(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCLOSEBRACKET, i)
}

func (s *AppnameContext) AllPLUS() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserPLUS)
}

func (s *AppnameContext) PLUS(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserPLUS, i)
}

func (s *AppnameContext) AllHYPHEN() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserHYPHEN)
}

func (s *AppnameContext) HYPHEN(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserHYPHEN, i)
}

func (s *AppnameContext) AllCOLON() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserCOLON)
}

func (s *AppnameContext) COLON(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCOLON, i)
}

func (s *AppnameContext) AllOPENSQUARE() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserOPENSQUARE)
}

func (s *AppnameContext) OPENSQUARE(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserOPENSQUARE, i)
}

func (s *AppnameContext) AllANTISLASH() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserANTISLASH)
}

func (s *AppnameContext) ANTISLASH(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserANTISLASH, i)
}

func (s *AppnameContext) AllEQUAL() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserEQUAL)
}

func (s *AppnameContext) EQUAL(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserEQUAL, i)
}

func (s *AppnameContext) AllCLOSESQUARE() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserCLOSESQUARE)
}

func (s *AppnameContext) CLOSESQUARE(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCLOSESQUARE, i)
}

func (s *AppnameContext) AllQUOTE() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserQUOTE)
}

func (s *AppnameContext) QUOTE(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserQUOTE, i)
}

func (s *AppnameContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *AppnameContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *AppnameContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterAppname(s)
	}
}

func (s *AppnameContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitAppname(s)
	}
}

func (p *RFC5424Parser) Appname() (localctx IAppnameContext) {
	localctx = NewAppnameContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 20, RFC5424ParserRULE_appname)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(153)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = (((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<RFC5424ParserDIGIT)|(1<<RFC5424ParserDOT)|(1<<RFC5424ParserOPENBRACKET)|(1<<RFC5424ParserCLOSEBRACKET)|(1<<RFC5424ParserPLUS)|(1<<RFC5424ParserHYPHEN)|(1<<RFC5424ParserCOLON)|(1<<RFC5424ParserEQUAL)|(1<<RFC5424ParserQUOTE)|(1<<RFC5424ParserOPENSQUARE)|(1<<RFC5424ParserCLOSESQUARE)|(1<<RFC5424ParserANTISLASH)|(1<<RFC5424ParserASCII))) != 0) {
		{
			p.SetState(152)
			_la = p.GetTokenStream().LA(1)

			if !(((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<RFC5424ParserDIGIT)|(1<<RFC5424ParserDOT)|(1<<RFC5424ParserOPENBRACKET)|(1<<RFC5424ParserCLOSEBRACKET)|(1<<RFC5424ParserPLUS)|(1<<RFC5424ParserHYPHEN)|(1<<RFC5424ParserCOLON)|(1<<RFC5424ParserEQUAL)|(1<<RFC5424ParserQUOTE)|(1<<RFC5424ParserOPENSQUARE)|(1<<RFC5424ParserCLOSESQUARE)|(1<<RFC5424ParserANTISLASH)|(1<<RFC5424ParserASCII))) != 0) {
				p.GetErrorHandler().RecoverInline(p)
			} else {
				p.GetErrorHandler().ReportMatch(p)
				p.Consume()
			}
		}

		p.SetState(155)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}

	return localctx
}

// IMsgidContext is an interface to support dynamic dispatch.
type IMsgidContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsMsgidContext differentiates from other interfaces.
	IsMsgidContext()
}

type MsgidContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyMsgidContext() *MsgidContext {
	var p = new(MsgidContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_msgid
	return p
}

func (*MsgidContext) IsMsgidContext() {}

func NewMsgidContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *MsgidContext {
	var p = new(MsgidContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_msgid

	return p
}

func (s *MsgidContext) GetParser() antlr.Parser { return s.parser }

func (s *MsgidContext) AllASCII() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserASCII)
}

func (s *MsgidContext) ASCII(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserASCII, i)
}

func (s *MsgidContext) AllDIGIT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDIGIT)
}

func (s *MsgidContext) DIGIT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, i)
}

func (s *MsgidContext) AllDOT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDOT)
}

func (s *MsgidContext) DOT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDOT, i)
}

func (s *MsgidContext) AllOPENBRACKET() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserOPENBRACKET)
}

func (s *MsgidContext) OPENBRACKET(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserOPENBRACKET, i)
}

func (s *MsgidContext) AllCLOSEBRACKET() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserCLOSEBRACKET)
}

func (s *MsgidContext) CLOSEBRACKET(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCLOSEBRACKET, i)
}

func (s *MsgidContext) AllPLUS() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserPLUS)
}

func (s *MsgidContext) PLUS(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserPLUS, i)
}

func (s *MsgidContext) AllHYPHEN() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserHYPHEN)
}

func (s *MsgidContext) HYPHEN(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserHYPHEN, i)
}

func (s *MsgidContext) AllCOLON() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserCOLON)
}

func (s *MsgidContext) COLON(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCOLON, i)
}

func (s *MsgidContext) AllOPENSQUARE() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserOPENSQUARE)
}

func (s *MsgidContext) OPENSQUARE(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserOPENSQUARE, i)
}

func (s *MsgidContext) AllANTISLASH() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserANTISLASH)
}

func (s *MsgidContext) ANTISLASH(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserANTISLASH, i)
}

func (s *MsgidContext) AllEQUAL() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserEQUAL)
}

func (s *MsgidContext) EQUAL(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserEQUAL, i)
}

func (s *MsgidContext) AllCLOSESQUARE() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserCLOSESQUARE)
}

func (s *MsgidContext) CLOSESQUARE(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCLOSESQUARE, i)
}

func (s *MsgidContext) AllQUOTE() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserQUOTE)
}

func (s *MsgidContext) QUOTE(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserQUOTE, i)
}

func (s *MsgidContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *MsgidContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *MsgidContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterMsgid(s)
	}
}

func (s *MsgidContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitMsgid(s)
	}
}

func (p *RFC5424Parser) Msgid() (localctx IMsgidContext) {
	localctx = NewMsgidContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 22, RFC5424ParserRULE_msgid)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(158)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = (((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<RFC5424ParserDIGIT)|(1<<RFC5424ParserDOT)|(1<<RFC5424ParserOPENBRACKET)|(1<<RFC5424ParserCLOSEBRACKET)|(1<<RFC5424ParserPLUS)|(1<<RFC5424ParserHYPHEN)|(1<<RFC5424ParserCOLON)|(1<<RFC5424ParserEQUAL)|(1<<RFC5424ParserQUOTE)|(1<<RFC5424ParserOPENSQUARE)|(1<<RFC5424ParserCLOSESQUARE)|(1<<RFC5424ParserANTISLASH)|(1<<RFC5424ParserASCII))) != 0) {
		{
			p.SetState(157)
			_la = p.GetTokenStream().LA(1)

			if !(((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<RFC5424ParserDIGIT)|(1<<RFC5424ParserDOT)|(1<<RFC5424ParserOPENBRACKET)|(1<<RFC5424ParserCLOSEBRACKET)|(1<<RFC5424ParserPLUS)|(1<<RFC5424ParserHYPHEN)|(1<<RFC5424ParserCOLON)|(1<<RFC5424ParserEQUAL)|(1<<RFC5424ParserQUOTE)|(1<<RFC5424ParserOPENSQUARE)|(1<<RFC5424ParserCLOSESQUARE)|(1<<RFC5424ParserANTISLASH)|(1<<RFC5424ParserASCII))) != 0) {
				p.GetErrorHandler().RecoverInline(p)
			} else {
				p.GetErrorHandler().ReportMatch(p)
				p.Consume()
			}
		}

		p.SetState(160)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}

	return localctx
}

// IProcidContext is an interface to support dynamic dispatch.
type IProcidContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsProcidContext differentiates from other interfaces.
	IsProcidContext()
}

type ProcidContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyProcidContext() *ProcidContext {
	var p = new(ProcidContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_procid
	return p
}

func (*ProcidContext) IsProcidContext() {}

func NewProcidContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ProcidContext {
	var p = new(ProcidContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_procid

	return p
}

func (s *ProcidContext) GetParser() antlr.Parser { return s.parser }

func (s *ProcidContext) AllASCII() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserASCII)
}

func (s *ProcidContext) ASCII(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserASCII, i)
}

func (s *ProcidContext) AllDIGIT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDIGIT)
}

func (s *ProcidContext) DIGIT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, i)
}

func (s *ProcidContext) AllDOT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDOT)
}

func (s *ProcidContext) DOT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDOT, i)
}

func (s *ProcidContext) AllOPENBRACKET() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserOPENBRACKET)
}

func (s *ProcidContext) OPENBRACKET(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserOPENBRACKET, i)
}

func (s *ProcidContext) AllCLOSEBRACKET() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserCLOSEBRACKET)
}

func (s *ProcidContext) CLOSEBRACKET(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCLOSEBRACKET, i)
}

func (s *ProcidContext) AllPLUS() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserPLUS)
}

func (s *ProcidContext) PLUS(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserPLUS, i)
}

func (s *ProcidContext) AllHYPHEN() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserHYPHEN)
}

func (s *ProcidContext) HYPHEN(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserHYPHEN, i)
}

func (s *ProcidContext) AllCOLON() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserCOLON)
}

func (s *ProcidContext) COLON(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCOLON, i)
}

func (s *ProcidContext) AllOPENSQUARE() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserOPENSQUARE)
}

func (s *ProcidContext) OPENSQUARE(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserOPENSQUARE, i)
}

func (s *ProcidContext) AllANTISLASH() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserANTISLASH)
}

func (s *ProcidContext) ANTISLASH(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserANTISLASH, i)
}

func (s *ProcidContext) AllEQUAL() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserEQUAL)
}

func (s *ProcidContext) EQUAL(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserEQUAL, i)
}

func (s *ProcidContext) AllCLOSESQUARE() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserCLOSESQUARE)
}

func (s *ProcidContext) CLOSESQUARE(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCLOSESQUARE, i)
}

func (s *ProcidContext) AllQUOTE() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserQUOTE)
}

func (s *ProcidContext) QUOTE(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserQUOTE, i)
}

func (s *ProcidContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ProcidContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ProcidContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterProcid(s)
	}
}

func (s *ProcidContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitProcid(s)
	}
}

func (p *RFC5424Parser) Procid() (localctx IProcidContext) {
	localctx = NewProcidContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 24, RFC5424ParserRULE_procid)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(163)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = (((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<RFC5424ParserDIGIT)|(1<<RFC5424ParserDOT)|(1<<RFC5424ParserOPENBRACKET)|(1<<RFC5424ParserCLOSEBRACKET)|(1<<RFC5424ParserPLUS)|(1<<RFC5424ParserHYPHEN)|(1<<RFC5424ParserCOLON)|(1<<RFC5424ParserEQUAL)|(1<<RFC5424ParserQUOTE)|(1<<RFC5424ParserOPENSQUARE)|(1<<RFC5424ParserCLOSESQUARE)|(1<<RFC5424ParserANTISLASH)|(1<<RFC5424ParserASCII))) != 0) {
		{
			p.SetState(162)
			_la = p.GetTokenStream().LA(1)

			if !(((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<RFC5424ParserDIGIT)|(1<<RFC5424ParserDOT)|(1<<RFC5424ParserOPENBRACKET)|(1<<RFC5424ParserCLOSEBRACKET)|(1<<RFC5424ParserPLUS)|(1<<RFC5424ParserHYPHEN)|(1<<RFC5424ParserCOLON)|(1<<RFC5424ParserEQUAL)|(1<<RFC5424ParserQUOTE)|(1<<RFC5424ParserOPENSQUARE)|(1<<RFC5424ParserCLOSESQUARE)|(1<<RFC5424ParserANTISLASH)|(1<<RFC5424ParserASCII))) != 0) {
				p.GetErrorHandler().RecoverInline(p)
			} else {
				p.GetErrorHandler().ReportMatch(p)
				p.Consume()
			}
		}

		p.SetState(165)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}

	return localctx
}

// IStructuredContext is an interface to support dynamic dispatch.
type IStructuredContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsStructuredContext differentiates from other interfaces.
	IsStructuredContext()
}

type StructuredContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyStructuredContext() *StructuredContext {
	var p = new(StructuredContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_structured
	return p
}

func (*StructuredContext) IsStructuredContext() {}

func NewStructuredContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *StructuredContext {
	var p = new(StructuredContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_structured

	return p
}

func (s *StructuredContext) GetParser() antlr.Parser { return s.parser }

func (s *StructuredContext) HYPHEN() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserHYPHEN, 0)
}

func (s *StructuredContext) AllElement() []IElementContext {
	var ts = s.GetTypedRuleContexts(reflect.TypeOf((*IElementContext)(nil)).Elem())
	var tst = make([]IElementContext, len(ts))

	for i, t := range ts {
		if t != nil {
			tst[i] = t.(IElementContext)
		}
	}

	return tst
}

func (s *StructuredContext) Element(i int) IElementContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IElementContext)(nil)).Elem(), i)

	if t == nil {
		return nil
	}

	return t.(IElementContext)
}

func (s *StructuredContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *StructuredContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *StructuredContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterStructured(s)
	}
}

func (s *StructuredContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitStructured(s)
	}
}

func (p *RFC5424Parser) Structured() (localctx IStructuredContext) {
	localctx = NewStructuredContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 26, RFC5424ParserRULE_structured)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.SetState(173)
	p.GetErrorHandler().Sync(p)

	switch p.GetTokenStream().LA(1) {
	case RFC5424ParserHYPHEN:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(167)
			p.Match(RFC5424ParserHYPHEN)
		}

	case RFC5424ParserOPENSQUARE:
		p.EnterOuterAlt(localctx, 2)
		p.SetState(169)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)

		for ok := true; ok; ok = _la == RFC5424ParserOPENSQUARE {
			{
				p.SetState(168)
				p.Element()
			}

			p.SetState(171)
			p.GetErrorHandler().Sync(p)
			_la = p.GetTokenStream().LA(1)
		}

	default:
		panic(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
	}

	return localctx
}

// IElementContext is an interface to support dynamic dispatch.
type IElementContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsElementContext differentiates from other interfaces.
	IsElementContext()
}

type ElementContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyElementContext() *ElementContext {
	var p = new(ElementContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_element
	return p
}

func (*ElementContext) IsElementContext() {}

func NewElementContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ElementContext {
	var p = new(ElementContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_element

	return p
}

func (s *ElementContext) GetParser() antlr.Parser { return s.parser }

func (s *ElementContext) OPENSQUARE() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserOPENSQUARE, 0)
}

func (s *ElementContext) Sid() ISidContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*ISidContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(ISidContext)
}

func (s *ElementContext) CLOSESQUARE() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCLOSESQUARE, 0)
}

func (s *ElementContext) AllSP() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserSP)
}

func (s *ElementContext) SP(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserSP, i)
}

func (s *ElementContext) AllParam() []IParamContext {
	var ts = s.GetTypedRuleContexts(reflect.TypeOf((*IParamContext)(nil)).Elem())
	var tst = make([]IParamContext, len(ts))

	for i, t := range ts {
		if t != nil {
			tst[i] = t.(IParamContext)
		}
	}

	return tst
}

func (s *ElementContext) Param(i int) IParamContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IParamContext)(nil)).Elem(), i)

	if t == nil {
		return nil
	}

	return t.(IParamContext)
}

func (s *ElementContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ElementContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ElementContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterElement(s)
	}
}

func (s *ElementContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitElement(s)
	}
}

func (p *RFC5424Parser) Element() (localctx IElementContext) {
	localctx = NewElementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 28, RFC5424ParserRULE_element)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(175)
		p.Match(RFC5424ParserOPENSQUARE)
	}
	{
		p.SetState(176)
		p.Sid()
	}
	p.SetState(181)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for _la == RFC5424ParserSP {
		{
			p.SetState(177)
			p.Match(RFC5424ParserSP)
		}
		{
			p.SetState(178)
			p.Param()
		}

		p.SetState(183)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(184)
		p.Match(RFC5424ParserCLOSESQUARE)
	}

	return localctx
}

// ISidContext is an interface to support dynamic dispatch.
type ISidContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsSidContext differentiates from other interfaces.
	IsSidContext()
}

type SidContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptySidContext() *SidContext {
	var p = new(SidContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_sid
	return p
}

func (*SidContext) IsSidContext() {}

func NewSidContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SidContext {
	var p = new(SidContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_sid

	return p
}

func (s *SidContext) GetParser() antlr.Parser { return s.parser }

func (s *SidContext) Name() INameContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*INameContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(INameContext)
}

func (s *SidContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SidContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *SidContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterSid(s)
	}
}

func (s *SidContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitSid(s)
	}
}

func (p *RFC5424Parser) Sid() (localctx ISidContext) {
	localctx = NewSidContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 30, RFC5424ParserRULE_sid)

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(186)
		p.Name()
	}

	return localctx
}

// IParamContext is an interface to support dynamic dispatch.
type IParamContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsParamContext differentiates from other interfaces.
	IsParamContext()
}

type ParamContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyParamContext() *ParamContext {
	var p = new(ParamContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_param
	return p
}

func (*ParamContext) IsParamContext() {}

func NewParamContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ParamContext {
	var p = new(ParamContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_param

	return p
}

func (s *ParamContext) GetParser() antlr.Parser { return s.parser }

func (s *ParamContext) Name() INameContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*INameContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(INameContext)
}

func (s *ParamContext) EQUAL() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserEQUAL, 0)
}

func (s *ParamContext) AllQUOTE() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserQUOTE)
}

func (s *ParamContext) QUOTE(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserQUOTE, i)
}

func (s *ParamContext) Value() IValueContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IValueContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IValueContext)
}

func (s *ParamContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ParamContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ParamContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterParam(s)
	}
}

func (s *ParamContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitParam(s)
	}
}

func (p *RFC5424Parser) Param() (localctx IParamContext) {
	localctx = NewParamContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 32, RFC5424ParserRULE_param)

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(188)
		p.Name()
	}
	{
		p.SetState(189)
		p.Match(RFC5424ParserEQUAL)
	}
	{
		p.SetState(190)
		p.Match(RFC5424ParserQUOTE)
	}
	{
		p.SetState(191)
		p.Value()
	}
	{
		p.SetState(192)
		p.Match(RFC5424ParserQUOTE)
	}

	return localctx
}

// INameContext is an interface to support dynamic dispatch.
type INameContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsNameContext differentiates from other interfaces.
	IsNameContext()
}

type NameContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyNameContext() *NameContext {
	var p = new(NameContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_name
	return p
}

func (*NameContext) IsNameContext() {}

func NewNameContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *NameContext {
	var p = new(NameContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_name

	return p
}

func (s *NameContext) GetParser() antlr.Parser { return s.parser }

func (s *NameContext) AllASCII() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserASCII)
}

func (s *NameContext) ASCII(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserASCII, i)
}

func (s *NameContext) AllDIGIT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDIGIT)
}

func (s *NameContext) DIGIT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, i)
}

func (s *NameContext) AllDOT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDOT)
}

func (s *NameContext) DOT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDOT, i)
}

func (s *NameContext) AllOPENBRACKET() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserOPENBRACKET)
}

func (s *NameContext) OPENBRACKET(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserOPENBRACKET, i)
}

func (s *NameContext) AllCLOSEBRACKET() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserCLOSEBRACKET)
}

func (s *NameContext) CLOSEBRACKET(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCLOSEBRACKET, i)
}

func (s *NameContext) AllPLUS() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserPLUS)
}

func (s *NameContext) PLUS(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserPLUS, i)
}

func (s *NameContext) AllHYPHEN() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserHYPHEN)
}

func (s *NameContext) HYPHEN(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserHYPHEN, i)
}

func (s *NameContext) AllCOLON() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserCOLON)
}

func (s *NameContext) COLON(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCOLON, i)
}

func (s *NameContext) AllOPENSQUARE() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserOPENSQUARE)
}

func (s *NameContext) OPENSQUARE(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserOPENSQUARE, i)
}

func (s *NameContext) AllANTISLASH() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserANTISLASH)
}

func (s *NameContext) ANTISLASH(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserANTISLASH, i)
}

func (s *NameContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *NameContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *NameContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterName(s)
	}
}

func (s *NameContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitName(s)
	}
}

func (p *RFC5424Parser) Name() (localctx INameContext) {
	localctx = NewNameContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 34, RFC5424ParserRULE_name)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(195)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = (((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<RFC5424ParserDIGIT)|(1<<RFC5424ParserDOT)|(1<<RFC5424ParserOPENBRACKET)|(1<<RFC5424ParserCLOSEBRACKET)|(1<<RFC5424ParserPLUS)|(1<<RFC5424ParserHYPHEN)|(1<<RFC5424ParserCOLON)|(1<<RFC5424ParserOPENSQUARE)|(1<<RFC5424ParserANTISLASH)|(1<<RFC5424ParserASCII))) != 0) {
		{
			p.SetState(194)
			_la = p.GetTokenStream().LA(1)

			if !(((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<RFC5424ParserDIGIT)|(1<<RFC5424ParserDOT)|(1<<RFC5424ParserOPENBRACKET)|(1<<RFC5424ParserCLOSEBRACKET)|(1<<RFC5424ParserPLUS)|(1<<RFC5424ParserHYPHEN)|(1<<RFC5424ParserCOLON)|(1<<RFC5424ParserOPENSQUARE)|(1<<RFC5424ParserANTISLASH)|(1<<RFC5424ParserASCII))) != 0) {
				p.GetErrorHandler().RecoverInline(p)
			} else {
				p.GetErrorHandler().ReportMatch(p)
				p.Consume()
			}
		}

		p.SetState(197)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}

	return localctx
}

// IValueContext is an interface to support dynamic dispatch.
type IValueContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsValueContext differentiates from other interfaces.
	IsValueContext()
}

type ValueContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyValueContext() *ValueContext {
	var p = new(ValueContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_value
	return p
}

func (*ValueContext) IsValueContext() {}

func NewValueContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ValueContext {
	var p = new(ValueContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_value

	return p
}

func (s *ValueContext) GetParser() antlr.Parser { return s.parser }

func (s *ValueContext) AllASCII() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserASCII)
}

func (s *ValueContext) ASCII(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserASCII, i)
}

func (s *ValueContext) AllVALUECHAR() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserVALUECHAR)
}

func (s *ValueContext) VALUECHAR(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserVALUECHAR, i)
}

func (s *ValueContext) AllDIGIT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDIGIT)
}

func (s *ValueContext) DIGIT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, i)
}

func (s *ValueContext) AllDOT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDOT)
}

func (s *ValueContext) DOT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDOT, i)
}

func (s *ValueContext) AllOPENBRACKET() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserOPENBRACKET)
}

func (s *ValueContext) OPENBRACKET(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserOPENBRACKET, i)
}

func (s *ValueContext) AllCLOSEBRACKET() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserCLOSEBRACKET)
}

func (s *ValueContext) CLOSEBRACKET(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCLOSEBRACKET, i)
}

func (s *ValueContext) AllPLUS() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserPLUS)
}

func (s *ValueContext) PLUS(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserPLUS, i)
}

func (s *ValueContext) AllHYPHEN() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserHYPHEN)
}

func (s *ValueContext) HYPHEN(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserHYPHEN, i)
}

func (s *ValueContext) AllCOLON() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserCOLON)
}

func (s *ValueContext) COLON(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCOLON, i)
}

func (s *ValueContext) AllSP() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserSP)
}

func (s *ValueContext) SP(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserSP, i)
}

func (s *ValueContext) AllEQUAL() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserEQUAL)
}

func (s *ValueContext) EQUAL(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserEQUAL, i)
}

func (s *ValueContext) AllOPENSQUARE() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserOPENSQUARE)
}

func (s *ValueContext) OPENSQUARE(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserOPENSQUARE, i)
}

func (s *ValueContext) AllANTISLASH() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserANTISLASH)
}

func (s *ValueContext) ANTISLASH(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserANTISLASH, i)
}

func (s *ValueContext) AllCLOSESQUARE() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserCLOSESQUARE)
}

func (s *ValueContext) CLOSESQUARE(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCLOSESQUARE, i)
}

func (s *ValueContext) AllQUOTE() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserQUOTE)
}

func (s *ValueContext) QUOTE(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserQUOTE, i)
}

func (s *ValueContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ValueContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ValueContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterValue(s)
	}
}

func (s *ValueContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitValue(s)
	}
}

func (p *RFC5424Parser) Value() (localctx IValueContext) {
	localctx = NewValueContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 36, RFC5424ParserRULE_value)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(219)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<RFC5424ParserDIGIT)|(1<<RFC5424ParserDOT)|(1<<RFC5424ParserOPENBRACKET)|(1<<RFC5424ParserCLOSEBRACKET)|(1<<RFC5424ParserPLUS)|(1<<RFC5424ParserHYPHEN)|(1<<RFC5424ParserCOLON)|(1<<RFC5424ParserSP)|(1<<RFC5424ParserEQUAL)|(1<<RFC5424ParserOPENSQUARE)|(1<<RFC5424ParserANTISLASH)|(1<<RFC5424ParserASCII)|(1<<RFC5424ParserVALUECHAR))) != 0 {
		p.SetState(217)
		p.GetErrorHandler().Sync(p)
		switch p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 22, p.GetParserRuleContext()) {
		case 1:
			{
				p.SetState(199)
				p.Match(RFC5424ParserASCII)
			}

		case 2:
			{
				p.SetState(200)
				p.Match(RFC5424ParserVALUECHAR)
			}

		case 3:
			{
				p.SetState(201)
				p.Match(RFC5424ParserDIGIT)
			}

		case 4:
			{
				p.SetState(202)
				p.Match(RFC5424ParserDOT)
			}

		case 5:
			{
				p.SetState(203)
				p.Match(RFC5424ParserOPENBRACKET)
			}

		case 6:
			{
				p.SetState(204)
				p.Match(RFC5424ParserCLOSEBRACKET)
			}

		case 7:
			{
				p.SetState(205)
				p.Match(RFC5424ParserPLUS)
			}

		case 8:
			{
				p.SetState(206)
				p.Match(RFC5424ParserHYPHEN)
			}

		case 9:
			{
				p.SetState(207)
				p.Match(RFC5424ParserCOLON)
			}

		case 10:
			{
				p.SetState(208)
				p.Match(RFC5424ParserSP)
			}

		case 11:
			{
				p.SetState(209)
				p.Match(RFC5424ParserEQUAL)
			}

		case 12:
			{
				p.SetState(210)
				p.Match(RFC5424ParserOPENSQUARE)
			}

		case 13:
			{
				p.SetState(211)
				p.Match(RFC5424ParserANTISLASH)
			}
			{
				p.SetState(212)
				p.Match(RFC5424ParserANTISLASH)
			}

		case 14:
			{
				p.SetState(213)
				p.Match(RFC5424ParserANTISLASH)
			}
			{
				p.SetState(214)
				p.Match(RFC5424ParserCLOSESQUARE)
			}

		case 15:
			{
				p.SetState(215)
				p.Match(RFC5424ParserANTISLASH)
			}
			{
				p.SetState(216)
				p.Match(RFC5424ParserQUOTE)
			}

		}

		p.SetState(221)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}

	return localctx
}
