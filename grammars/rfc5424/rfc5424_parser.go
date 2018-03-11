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
	3, 24715, 42794, 33075, 47597, 16764, 15335, 30598, 22884, 3, 18, 288,
	4, 2, 9, 2, 4, 3, 9, 3, 4, 4, 9, 4, 4, 5, 9, 5, 4, 6, 9, 6, 4, 7, 9, 7,
	4, 8, 9, 8, 4, 9, 9, 9, 4, 10, 9, 10, 4, 11, 9, 11, 4, 12, 9, 12, 4, 13,
	9, 13, 4, 14, 9, 14, 4, 15, 9, 15, 4, 16, 9, 16, 4, 17, 9, 17, 4, 18, 9,
	18, 4, 19, 9, 19, 4, 20, 9, 20, 4, 21, 9, 21, 4, 22, 9, 22, 4, 23, 9, 23,
	4, 24, 9, 24, 4, 25, 9, 25, 4, 26, 9, 26, 4, 27, 9, 27, 4, 28, 9, 28, 4,
	29, 9, 29, 4, 30, 9, 30, 4, 31, 9, 31, 4, 32, 9, 32, 3, 2, 3, 2, 6, 2,
	67, 10, 2, 13, 2, 14, 2, 68, 3, 2, 3, 2, 6, 2, 73, 10, 2, 13, 2, 14, 2,
	74, 3, 2, 5, 2, 78, 10, 2, 3, 2, 3, 2, 3, 3, 3, 3, 3, 3, 6, 3, 85, 10,
	3, 13, 3, 14, 3, 86, 3, 3, 3, 3, 6, 3, 91, 10, 3, 13, 3, 14, 3, 92, 3,
	3, 3, 3, 6, 3, 97, 10, 3, 13, 3, 14, 3, 98, 3, 3, 3, 3, 6, 3, 103, 10,
	3, 13, 3, 14, 3, 104, 3, 3, 3, 3, 6, 3, 109, 10, 3, 13, 3, 14, 3, 110,
	3, 3, 3, 3, 3, 4, 7, 4, 116, 10, 4, 12, 4, 14, 4, 119, 11, 4, 3, 5, 3,
	5, 3, 5, 3, 5, 3, 5, 3, 5, 3, 5, 5, 5, 128, 10, 5, 3, 6, 3, 6, 3, 6, 3,
	6, 3, 6, 3, 7, 3, 7, 3, 7, 3, 7, 3, 7, 3, 7, 3, 8, 3, 8, 3, 8, 3, 8, 3,
	8, 3, 9, 3, 9, 3, 9, 3, 10, 3, 10, 3, 10, 3, 11, 3, 11, 3, 11, 3, 11, 3,
	11, 3, 11, 3, 11, 5, 11, 159, 10, 11, 3, 12, 3, 12, 3, 12, 3, 13, 3, 13,
	3, 13, 3, 14, 3, 14, 3, 14, 3, 15, 6, 15, 171, 10, 15, 13, 15, 14, 15,
	172, 3, 16, 3, 16, 5, 16, 177, 10, 16, 3, 17, 3, 17, 3, 17, 3, 17, 3, 17,
	3, 18, 3, 18, 6, 18, 186, 10, 18, 13, 18, 14, 18, 187, 3, 18, 3, 18, 3,
	19, 3, 19, 3, 20, 3, 20, 3, 20, 6, 20, 197, 10, 20, 13, 20, 14, 20, 198,
	5, 20, 201, 10, 20, 3, 21, 3, 21, 3, 21, 6, 21, 206, 10, 21, 13, 21, 14,
	21, 207, 5, 21, 210, 10, 21, 3, 22, 3, 22, 3, 22, 6, 22, 215, 10, 22, 13,
	22, 14, 22, 216, 5, 22, 219, 10, 22, 3, 23, 3, 23, 3, 23, 6, 23, 224, 10,
	23, 13, 23, 14, 23, 225, 5, 23, 228, 10, 23, 3, 24, 3, 24, 3, 24, 3, 24,
	3, 24, 5, 24, 235, 10, 24, 3, 25, 3, 25, 6, 25, 239, 10, 25, 13, 25, 14,
	25, 240, 5, 25, 243, 10, 25, 3, 26, 3, 26, 3, 26, 3, 26, 7, 26, 249, 10,
	26, 12, 26, 14, 26, 252, 11, 26, 3, 26, 3, 26, 3, 27, 3, 27, 3, 28, 3,
	28, 3, 28, 3, 28, 3, 28, 3, 28, 3, 29, 3, 29, 6, 29, 266, 10, 29, 13, 29,
	14, 29, 267, 3, 30, 3, 30, 3, 31, 3, 31, 3, 31, 3, 31, 3, 31, 3, 31, 3,
	31, 3, 31, 3, 31, 7, 31, 281, 10, 31, 12, 31, 14, 31, 284, 11, 31, 3, 32,
	3, 32, 3, 32, 3, 117, 2, 33, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22, 24,
	26, 28, 30, 32, 34, 36, 38, 40, 42, 44, 46, 48, 50, 52, 54, 56, 58, 60,
	62, 2, 5, 3, 2, 7, 8, 5, 2, 3, 9, 13, 13, 15, 15, 4, 2, 3, 11, 13, 13,
	2, 298, 2, 64, 3, 2, 2, 2, 4, 81, 3, 2, 2, 2, 6, 117, 3, 2, 2, 2, 8, 127,
	3, 2, 2, 2, 10, 129, 3, 2, 2, 2, 12, 134, 3, 2, 2, 2, 14, 140, 3, 2, 2,
	2, 16, 145, 3, 2, 2, 2, 18, 148, 3, 2, 2, 2, 20, 151, 3, 2, 2, 2, 22, 160,
	3, 2, 2, 2, 24, 163, 3, 2, 2, 2, 26, 166, 3, 2, 2, 2, 28, 170, 3, 2, 2,
	2, 30, 176, 3, 2, 2, 2, 32, 178, 3, 2, 2, 2, 34, 183, 3, 2, 2, 2, 36, 191,
	3, 2, 2, 2, 38, 200, 3, 2, 2, 2, 40, 209, 3, 2, 2, 2, 42, 218, 3, 2, 2,
	2, 44, 227, 3, 2, 2, 2, 46, 234, 3, 2, 2, 2, 48, 242, 3, 2, 2, 2, 50, 244,
	3, 2, 2, 2, 52, 255, 3, 2, 2, 2, 54, 257, 3, 2, 2, 2, 56, 265, 3, 2, 2,
	2, 58, 269, 3, 2, 2, 2, 60, 282, 3, 2, 2, 2, 62, 285, 3, 2, 2, 2, 64, 66,
	5, 4, 3, 2, 65, 67, 7, 10, 2, 2, 66, 65, 3, 2, 2, 2, 67, 68, 3, 2, 2, 2,
	68, 66, 3, 2, 2, 2, 68, 69, 3, 2, 2, 2, 69, 70, 3, 2, 2, 2, 70, 77, 5,
	48, 25, 2, 71, 73, 7, 10, 2, 2, 72, 71, 3, 2, 2, 2, 73, 74, 3, 2, 2, 2,
	74, 72, 3, 2, 2, 2, 74, 75, 3, 2, 2, 2, 75, 76, 3, 2, 2, 2, 76, 78, 5,
	6, 4, 2, 77, 72, 3, 2, 2, 2, 77, 78, 3, 2, 2, 2, 78, 79, 3, 2, 2, 2, 79,
	80, 7, 2, 2, 3, 80, 3, 3, 2, 2, 2, 81, 82, 5, 34, 18, 2, 82, 84, 5, 36,
	19, 2, 83, 85, 7, 10, 2, 2, 84, 83, 3, 2, 2, 2, 85, 86, 3, 2, 2, 2, 86,
	84, 3, 2, 2, 2, 86, 87, 3, 2, 2, 2, 87, 88, 3, 2, 2, 2, 88, 90, 5, 10,
	6, 2, 89, 91, 7, 10, 2, 2, 90, 89, 3, 2, 2, 2, 91, 92, 3, 2, 2, 2, 92,
	90, 3, 2, 2, 2, 92, 93, 3, 2, 2, 2, 93, 94, 3, 2, 2, 2, 94, 96, 5, 38,
	20, 2, 95, 97, 7, 10, 2, 2, 96, 95, 3, 2, 2, 2, 97, 98, 3, 2, 2, 2, 98,
	96, 3, 2, 2, 2, 98, 99, 3, 2, 2, 2, 99, 100, 3, 2, 2, 2, 100, 102, 5, 40,
	21, 2, 101, 103, 7, 10, 2, 2, 102, 101, 3, 2, 2, 2, 103, 104, 3, 2, 2,
	2, 104, 102, 3, 2, 2, 2, 104, 105, 3, 2, 2, 2, 105, 106, 3, 2, 2, 2, 106,
	108, 5, 44, 23, 2, 107, 109, 7, 10, 2, 2, 108, 107, 3, 2, 2, 2, 109, 110,
	3, 2, 2, 2, 110, 108, 3, 2, 2, 2, 110, 111, 3, 2, 2, 2, 111, 112, 3, 2,
	2, 2, 112, 113, 5, 42, 22, 2, 113, 5, 3, 2, 2, 2, 114, 116, 5, 8, 5, 2,
	115, 114, 3, 2, 2, 2, 116, 119, 3, 2, 2, 2, 117, 118, 3, 2, 2, 2, 117,
	115, 3, 2, 2, 2, 118, 7, 3, 2, 2, 2, 119, 117, 3, 2, 2, 2, 120, 128, 7,
	16, 2, 2, 121, 128, 7, 17, 2, 2, 122, 128, 5, 62, 32, 2, 123, 128, 7, 15,
	2, 2, 124, 128, 7, 14, 2, 2, 125, 128, 7, 12, 2, 2, 126, 128, 7, 18, 2,
	2, 127, 120, 3, 2, 2, 2, 127, 121, 3, 2, 2, 2, 127, 122, 3, 2, 2, 2, 127,
	123, 3, 2, 2, 2, 127, 124, 3, 2, 2, 2, 127, 125, 3, 2, 2, 2, 127, 126,
	3, 2, 2, 2, 128, 9, 3, 2, 2, 2, 129, 130, 5, 12, 7, 2, 130, 131, 7, 16,
	2, 2, 131, 132, 5, 20, 11, 2, 132, 133, 5, 30, 16, 2, 133, 11, 3, 2, 2,
	2, 134, 135, 5, 14, 8, 2, 135, 136, 7, 8, 2, 2, 136, 137, 5, 16, 9, 2,
	137, 138, 7, 8, 2, 2, 138, 139, 5, 18, 10, 2, 139, 13, 3, 2, 2, 2, 140,
	141, 7, 3, 2, 2, 141, 142, 7, 3, 2, 2, 142, 143, 7, 3, 2, 2, 143, 144,
	7, 3, 2, 2, 144, 15, 3, 2, 2, 2, 145, 146, 7, 3, 2, 2, 146, 147, 7, 3,
	2, 2, 147, 17, 3, 2, 2, 2, 148, 149, 7, 3, 2, 2, 149, 150, 7, 3, 2, 2,
	150, 19, 3, 2, 2, 2, 151, 152, 5, 22, 12, 2, 152, 153, 7, 9, 2, 2, 153,
	154, 5, 24, 13, 2, 154, 155, 7, 9, 2, 2, 155, 158, 5, 26, 14, 2, 156, 157,
	7, 4, 2, 2, 157, 159, 5, 28, 15, 2, 158, 156, 3, 2, 2, 2, 158, 159, 3,
	2, 2, 2, 159, 21, 3, 2, 2, 2, 160, 161, 7, 3, 2, 2, 161, 162, 7, 3, 2,
	2, 162, 23, 3, 2, 2, 2, 163, 164, 7, 3, 2, 2, 164, 165, 7, 3, 2, 2, 165,
	25, 3, 2, 2, 2, 166, 167, 7, 3, 2, 2, 167, 168, 7, 3, 2, 2, 168, 27, 3,
	2, 2, 2, 169, 171, 7, 3, 2, 2, 170, 169, 3, 2, 2, 2, 171, 172, 3, 2, 2,
	2, 172, 170, 3, 2, 2, 2, 172, 173, 3, 2, 2, 2, 173, 29, 3, 2, 2, 2, 174,
	177, 7, 16, 2, 2, 175, 177, 5, 32, 17, 2, 176, 174, 3, 2, 2, 2, 176, 175,
	3, 2, 2, 2, 177, 31, 3, 2, 2, 2, 178, 179, 9, 2, 2, 2, 179, 180, 5, 22,
	12, 2, 180, 181, 7, 9, 2, 2, 181, 182, 5, 24, 13, 2, 182, 33, 3, 2, 2,
	2, 183, 185, 7, 5, 2, 2, 184, 186, 7, 3, 2, 2, 185, 184, 3, 2, 2, 2, 186,
	187, 3, 2, 2, 2, 187, 185, 3, 2, 2, 2, 187, 188, 3, 2, 2, 2, 188, 189,
	3, 2, 2, 2, 189, 190, 7, 6, 2, 2, 190, 35, 3, 2, 2, 2, 191, 192, 7, 3,
	2, 2, 192, 37, 3, 2, 2, 2, 193, 201, 7, 8, 2, 2, 194, 196, 5, 46, 24, 2,
	195, 197, 5, 46, 24, 2, 196, 195, 3, 2, 2, 2, 197, 198, 3, 2, 2, 2, 198,
	196, 3, 2, 2, 2, 198, 199, 3, 2, 2, 2, 199, 201, 3, 2, 2, 2, 200, 193,
	3, 2, 2, 2, 200, 194, 3, 2, 2, 2, 201, 39, 3, 2, 2, 2, 202, 210, 7, 8,
	2, 2, 203, 205, 5, 46, 24, 2, 204, 206, 5, 46, 24, 2, 205, 204, 3, 2, 2,
	2, 206, 207, 3, 2, 2, 2, 207, 205, 3, 2, 2, 2, 207, 208, 3, 2, 2, 2, 208,
	210, 3, 2, 2, 2, 209, 202, 3, 2, 2, 2, 209, 203, 3, 2, 2, 2, 210, 41, 3,
	2, 2, 2, 211, 219, 7, 8, 2, 2, 212, 214, 5, 46, 24, 2, 213, 215, 5, 46,
	24, 2, 214, 213, 3, 2, 2, 2, 215, 216, 3, 2, 2, 2, 216, 214, 3, 2, 2, 2,
	216, 217, 3, 2, 2, 2, 217, 219, 3, 2, 2, 2, 218, 211, 3, 2, 2, 2, 218,
	212, 3, 2, 2, 2, 219, 43, 3, 2, 2, 2, 220, 228, 7, 8, 2, 2, 221, 223, 5,
	46, 24, 2, 222, 224, 5, 46, 24, 2, 223, 222, 3, 2, 2, 2, 224, 225, 3, 2,
	2, 2, 225, 223, 3, 2, 2, 2, 225, 226, 3, 2, 2, 2, 226, 228, 3, 2, 2, 2,
	227, 220, 3, 2, 2, 2, 227, 221, 3, 2, 2, 2, 228, 45, 3, 2, 2, 2, 229, 235,
	7, 16, 2, 2, 230, 235, 5, 58, 30, 2, 231, 235, 7, 11, 2, 2, 232, 235, 7,
	14, 2, 2, 233, 235, 7, 12, 2, 2, 234, 229, 3, 2, 2, 2, 234, 230, 3, 2,
	2, 2, 234, 231, 3, 2, 2, 2, 234, 232, 3, 2, 2, 2, 234, 233, 3, 2, 2, 2,
	235, 47, 3, 2, 2, 2, 236, 243, 7, 8, 2, 2, 237, 239, 5, 50, 26, 2, 238,
	237, 3, 2, 2, 2, 239, 240, 3, 2, 2, 2, 240, 238, 3, 2, 2, 2, 240, 241,
	3, 2, 2, 2, 241, 243, 3, 2, 2, 2, 242, 236, 3, 2, 2, 2, 242, 238, 3, 2,
	2, 2, 243, 49, 3, 2, 2, 2, 244, 245, 7, 13, 2, 2, 245, 250, 5, 52, 27,
	2, 246, 247, 7, 10, 2, 2, 247, 249, 5, 54, 28, 2, 248, 246, 3, 2, 2, 2,
	249, 252, 3, 2, 2, 2, 250, 248, 3, 2, 2, 2, 250, 251, 3, 2, 2, 2, 251,
	253, 3, 2, 2, 2, 252, 250, 3, 2, 2, 2, 253, 254, 7, 14, 2, 2, 254, 51,
	3, 2, 2, 2, 255, 256, 5, 56, 29, 2, 256, 53, 3, 2, 2, 2, 257, 258, 5, 56,
	29, 2, 258, 259, 7, 11, 2, 2, 259, 260, 7, 12, 2, 2, 260, 261, 5, 60, 31,
	2, 261, 262, 7, 12, 2, 2, 262, 55, 3, 2, 2, 2, 263, 266, 7, 16, 2, 2, 264,
	266, 5, 58, 30, 2, 265, 263, 3, 2, 2, 2, 265, 264, 3, 2, 2, 2, 266, 267,
	3, 2, 2, 2, 267, 265, 3, 2, 2, 2, 267, 268, 3, 2, 2, 2, 268, 57, 3, 2,
	2, 2, 269, 270, 9, 3, 2, 2, 270, 59, 3, 2, 2, 2, 271, 281, 7, 16, 2, 2,
	272, 281, 7, 17, 2, 2, 273, 281, 5, 62, 32, 2, 274, 275, 7, 15, 2, 2, 275,
	281, 7, 15, 2, 2, 276, 277, 7, 15, 2, 2, 277, 281, 7, 14, 2, 2, 278, 279,
	7, 15, 2, 2, 279, 281, 7, 12, 2, 2, 280, 271, 3, 2, 2, 2, 280, 272, 3,
	2, 2, 2, 280, 273, 3, 2, 2, 2, 280, 274, 3, 2, 2, 2, 280, 276, 3, 2, 2,
	2, 280, 278, 3, 2, 2, 2, 281, 284, 3, 2, 2, 2, 282, 280, 3, 2, 2, 2, 282,
	283, 3, 2, 2, 2, 283, 61, 3, 2, 2, 2, 284, 282, 3, 2, 2, 2, 285, 286, 9,
	4, 2, 2, 286, 63, 3, 2, 2, 2, 32, 68, 74, 77, 86, 92, 98, 104, 110, 117,
	127, 158, 172, 176, 187, 198, 200, 207, 209, 216, 218, 225, 227, 234, 240,
	242, 250, 265, 267, 280, 282,
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
	"full", "headr", "msg", "allchars", "timestamp", "date", "year", "month",
	"day", "time", "hour", "minute", "second", "nano", "timezone", "timezonenum",
	"pri", "version", "hostname", "appname", "msgid", "procid", "allascii",
	"structured", "element", "sid", "param", "name", "specialname", "value",
	"specialvalue",
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
	RFC5424ParserRULE_full         = 0
	RFC5424ParserRULE_headr        = 1
	RFC5424ParserRULE_msg          = 2
	RFC5424ParserRULE_allchars     = 3
	RFC5424ParserRULE_timestamp    = 4
	RFC5424ParserRULE_date         = 5
	RFC5424ParserRULE_year         = 6
	RFC5424ParserRULE_month        = 7
	RFC5424ParserRULE_day          = 8
	RFC5424ParserRULE_time         = 9
	RFC5424ParserRULE_hour         = 10
	RFC5424ParserRULE_minute       = 11
	RFC5424ParserRULE_second       = 12
	RFC5424ParserRULE_nano         = 13
	RFC5424ParserRULE_timezone     = 14
	RFC5424ParserRULE_timezonenum  = 15
	RFC5424ParserRULE_pri          = 16
	RFC5424ParserRULE_version      = 17
	RFC5424ParserRULE_hostname     = 18
	RFC5424ParserRULE_appname      = 19
	RFC5424ParserRULE_msgid        = 20
	RFC5424ParserRULE_procid       = 21
	RFC5424ParserRULE_allascii     = 22
	RFC5424ParserRULE_structured   = 23
	RFC5424ParserRULE_element      = 24
	RFC5424ParserRULE_sid          = 25
	RFC5424ParserRULE_param        = 26
	RFC5424ParserRULE_name         = 27
	RFC5424ParserRULE_specialname  = 28
	RFC5424ParserRULE_value        = 29
	RFC5424ParserRULE_specialvalue = 30
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
		p.SetState(62)
		p.Headr()
	}
	p.SetState(64)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = _la == RFC5424ParserSP {
		{
			p.SetState(63)
			p.Match(RFC5424ParserSP)
		}

		p.SetState(66)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(68)
		p.Structured()
	}
	p.SetState(75)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	if _la == RFC5424ParserSP {
		p.SetState(70)
		p.GetErrorHandler().Sync(p)
		_alt = 1
		for ok := true; ok; ok = _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
			switch _alt {
			case 1:
				{
					p.SetState(69)
					p.Match(RFC5424ParserSP)
				}

			default:
				panic(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
			}

			p.SetState(72)
			p.GetErrorHandler().Sync(p)
			_alt = p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 1, p.GetParserRuleContext())
		}
		{
			p.SetState(74)
			p.Msg()
		}

	}
	{
		p.SetState(77)
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

func (s *HeadrContext) Pri() IPriContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IPriContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IPriContext)
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
		p.SetState(79)
		p.Pri()
	}
	{
		p.SetState(80)
		p.Version()
	}
	p.SetState(82)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = _la == RFC5424ParserSP {
		{
			p.SetState(81)
			p.Match(RFC5424ParserSP)
		}

		p.SetState(84)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(86)
		p.Timestamp()
	}
	p.SetState(88)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = _la == RFC5424ParserSP {
		{
			p.SetState(87)
			p.Match(RFC5424ParserSP)
		}

		p.SetState(90)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(92)
		p.Hostname()
	}
	p.SetState(94)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = _la == RFC5424ParserSP {
		{
			p.SetState(93)
			p.Match(RFC5424ParserSP)
		}

		p.SetState(96)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(98)
		p.Appname()
	}
	p.SetState(100)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = _la == RFC5424ParserSP {
		{
			p.SetState(99)
			p.Match(RFC5424ParserSP)
		}

		p.SetState(102)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(104)
		p.Procid()
	}
	p.SetState(106)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = _la == RFC5424ParserSP {
		{
			p.SetState(105)
			p.Match(RFC5424ParserSP)
		}

		p.SetState(108)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(110)
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

func (s *MsgContext) AllAllchars() []IAllcharsContext {
	var ts = s.GetTypedRuleContexts(reflect.TypeOf((*IAllcharsContext)(nil)).Elem())
	var tst = make([]IAllcharsContext, len(ts))

	for i, t := range ts {
		if t != nil {
			tst[i] = t.(IAllcharsContext)
		}
	}

	return tst
}

func (s *MsgContext) Allchars(i int) IAllcharsContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IAllcharsContext)(nil)).Elem(), i)

	if t == nil {
		return nil
	}

	return t.(IAllcharsContext)
}

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
	p.SetState(115)
	p.GetErrorHandler().Sync(p)
	_alt = p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 8, p.GetParserRuleContext())

	for _alt != 1 && _alt != antlr.ATNInvalidAltNumber {
		if _alt == 1+1 {
			{
				p.SetState(112)
				p.Allchars()
			}

		}
		p.SetState(117)
		p.GetErrorHandler().Sync(p)
		_alt = p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 8, p.GetParserRuleContext())
	}

	return localctx
}

// IAllcharsContext is an interface to support dynamic dispatch.
type IAllcharsContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsAllcharsContext differentiates from other interfaces.
	IsAllcharsContext()
}

type AllcharsContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyAllcharsContext() *AllcharsContext {
	var p = new(AllcharsContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_allchars
	return p
}

func (*AllcharsContext) IsAllcharsContext() {}

func NewAllcharsContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *AllcharsContext {
	var p = new(AllcharsContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_allchars

	return p
}

func (s *AllcharsContext) GetParser() antlr.Parser { return s.parser }

func (s *AllcharsContext) ASCII() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserASCII, 0)
}

func (s *AllcharsContext) VALUECHAR() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserVALUECHAR, 0)
}

func (s *AllcharsContext) Specialvalue() ISpecialvalueContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*ISpecialvalueContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(ISpecialvalueContext)
}

func (s *AllcharsContext) ANTISLASH() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserANTISLASH, 0)
}

func (s *AllcharsContext) CLOSESQUARE() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCLOSESQUARE, 0)
}

func (s *AllcharsContext) QUOTE() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserQUOTE, 0)
}

func (s *AllcharsContext) ANYCHAR() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserANYCHAR, 0)
}

func (s *AllcharsContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *AllcharsContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *AllcharsContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterAllchars(s)
	}
}

func (s *AllcharsContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitAllchars(s)
	}
}

func (p *RFC5424Parser) Allchars() (localctx IAllcharsContext) {
	localctx = NewAllcharsContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 6, RFC5424ParserRULE_allchars)

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

	p.SetState(125)
	p.GetErrorHandler().Sync(p)

	switch p.GetTokenStream().LA(1) {
	case RFC5424ParserASCII:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(118)
			p.Match(RFC5424ParserASCII)
		}

	case RFC5424ParserVALUECHAR:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(119)
			p.Match(RFC5424ParserVALUECHAR)
		}

	case RFC5424ParserDIGIT, RFC5424ParserDOT, RFC5424ParserOPENBRACKET, RFC5424ParserCLOSEBRACKET, RFC5424ParserPLUS, RFC5424ParserHYPHEN, RFC5424ParserCOLON, RFC5424ParserSP, RFC5424ParserEQUAL, RFC5424ParserOPENSQUARE:
		p.EnterOuterAlt(localctx, 3)
		{
			p.SetState(120)
			p.Specialvalue()
		}

	case RFC5424ParserANTISLASH:
		p.EnterOuterAlt(localctx, 4)
		{
			p.SetState(121)
			p.Match(RFC5424ParserANTISLASH)
		}

	case RFC5424ParserCLOSESQUARE:
		p.EnterOuterAlt(localctx, 5)
		{
			p.SetState(122)
			p.Match(RFC5424ParserCLOSESQUARE)
		}

	case RFC5424ParserQUOTE:
		p.EnterOuterAlt(localctx, 6)
		{
			p.SetState(123)
			p.Match(RFC5424ParserQUOTE)
		}

	case RFC5424ParserANYCHAR:
		p.EnterOuterAlt(localctx, 7)
		{
			p.SetState(124)
			p.Match(RFC5424ParserANYCHAR)
		}

	default:
		panic(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
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
	p.EnterRule(localctx, 8, RFC5424ParserRULE_timestamp)

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
		p.SetState(127)
		p.Date()
	}
	{
		p.SetState(128)
		p.Match(RFC5424ParserASCII)
	}
	{
		p.SetState(129)
		p.Time()
	}
	{
		p.SetState(130)
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

func (s *DateContext) Year() IYearContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IYearContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IYearContext)
}

func (s *DateContext) AllHYPHEN() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserHYPHEN)
}

func (s *DateContext) HYPHEN(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserHYPHEN, i)
}

func (s *DateContext) Month() IMonthContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IMonthContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IMonthContext)
}

func (s *DateContext) Day() IDayContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IDayContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IDayContext)
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
	p.EnterRule(localctx, 10, RFC5424ParserRULE_date)

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
		p.SetState(132)
		p.Year()
	}
	{
		p.SetState(133)
		p.Match(RFC5424ParserHYPHEN)
	}
	{
		p.SetState(134)
		p.Month()
	}
	{
		p.SetState(135)
		p.Match(RFC5424ParserHYPHEN)
	}
	{
		p.SetState(136)
		p.Day()
	}

	return localctx
}

// IYearContext is an interface to support dynamic dispatch.
type IYearContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsYearContext differentiates from other interfaces.
	IsYearContext()
}

type YearContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyYearContext() *YearContext {
	var p = new(YearContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_year
	return p
}

func (*YearContext) IsYearContext() {}

func NewYearContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *YearContext {
	var p = new(YearContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_year

	return p
}

func (s *YearContext) GetParser() antlr.Parser { return s.parser }

func (s *YearContext) AllDIGIT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDIGIT)
}

func (s *YearContext) DIGIT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, i)
}

func (s *YearContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *YearContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *YearContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterYear(s)
	}
}

func (s *YearContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitYear(s)
	}
}

func (p *RFC5424Parser) Year() (localctx IYearContext) {
	localctx = NewYearContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 12, RFC5424ParserRULE_year)

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
		p.SetState(138)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(139)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(140)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(141)
		p.Match(RFC5424ParserDIGIT)
	}

	return localctx
}

// IMonthContext is an interface to support dynamic dispatch.
type IMonthContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsMonthContext differentiates from other interfaces.
	IsMonthContext()
}

type MonthContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyMonthContext() *MonthContext {
	var p = new(MonthContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_month
	return p
}

func (*MonthContext) IsMonthContext() {}

func NewMonthContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *MonthContext {
	var p = new(MonthContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_month

	return p
}

func (s *MonthContext) GetParser() antlr.Parser { return s.parser }

func (s *MonthContext) AllDIGIT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDIGIT)
}

func (s *MonthContext) DIGIT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, i)
}

func (s *MonthContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *MonthContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *MonthContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterMonth(s)
	}
}

func (s *MonthContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitMonth(s)
	}
}

func (p *RFC5424Parser) Month() (localctx IMonthContext) {
	localctx = NewMonthContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 14, RFC5424ParserRULE_month)

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
		p.SetState(143)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(144)
		p.Match(RFC5424ParserDIGIT)
	}

	return localctx
}

// IDayContext is an interface to support dynamic dispatch.
type IDayContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsDayContext differentiates from other interfaces.
	IsDayContext()
}

type DayContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyDayContext() *DayContext {
	var p = new(DayContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_day
	return p
}

func (*DayContext) IsDayContext() {}

func NewDayContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *DayContext {
	var p = new(DayContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_day

	return p
}

func (s *DayContext) GetParser() antlr.Parser { return s.parser }

func (s *DayContext) AllDIGIT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDIGIT)
}

func (s *DayContext) DIGIT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, i)
}

func (s *DayContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *DayContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *DayContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterDay(s)
	}
}

func (s *DayContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitDay(s)
	}
}

func (p *RFC5424Parser) Day() (localctx IDayContext) {
	localctx = NewDayContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 16, RFC5424ParserRULE_day)

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
		p.SetState(146)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(147)
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

func (s *TimeContext) Hour() IHourContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IHourContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IHourContext)
}

func (s *TimeContext) AllCOLON() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserCOLON)
}

func (s *TimeContext) COLON(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCOLON, i)
}

func (s *TimeContext) Minute() IMinuteContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IMinuteContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IMinuteContext)
}

func (s *TimeContext) Second() ISecondContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*ISecondContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(ISecondContext)
}

func (s *TimeContext) DOT() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDOT, 0)
}

func (s *TimeContext) Nano() INanoContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*INanoContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(INanoContext)
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
	p.EnterRule(localctx, 18, RFC5424ParserRULE_time)
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
		p.SetState(149)
		p.Hour()
	}
	{
		p.SetState(150)
		p.Match(RFC5424ParserCOLON)
	}
	{
		p.SetState(151)
		p.Minute()
	}
	{
		p.SetState(152)
		p.Match(RFC5424ParserCOLON)
	}
	{
		p.SetState(153)
		p.Second()
	}
	p.SetState(156)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	if _la == RFC5424ParserDOT {
		{
			p.SetState(154)
			p.Match(RFC5424ParserDOT)
		}
		{
			p.SetState(155)
			p.Nano()
		}

	}

	return localctx
}

// IHourContext is an interface to support dynamic dispatch.
type IHourContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsHourContext differentiates from other interfaces.
	IsHourContext()
}

type HourContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyHourContext() *HourContext {
	var p = new(HourContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_hour
	return p
}

func (*HourContext) IsHourContext() {}

func NewHourContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *HourContext {
	var p = new(HourContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_hour

	return p
}

func (s *HourContext) GetParser() antlr.Parser { return s.parser }

func (s *HourContext) AllDIGIT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDIGIT)
}

func (s *HourContext) DIGIT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, i)
}

func (s *HourContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *HourContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *HourContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterHour(s)
	}
}

func (s *HourContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitHour(s)
	}
}

func (p *RFC5424Parser) Hour() (localctx IHourContext) {
	localctx = NewHourContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 20, RFC5424ParserRULE_hour)

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
		p.SetState(158)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(159)
		p.Match(RFC5424ParserDIGIT)
	}

	return localctx
}

// IMinuteContext is an interface to support dynamic dispatch.
type IMinuteContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsMinuteContext differentiates from other interfaces.
	IsMinuteContext()
}

type MinuteContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyMinuteContext() *MinuteContext {
	var p = new(MinuteContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_minute
	return p
}

func (*MinuteContext) IsMinuteContext() {}

func NewMinuteContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *MinuteContext {
	var p = new(MinuteContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_minute

	return p
}

func (s *MinuteContext) GetParser() antlr.Parser { return s.parser }

func (s *MinuteContext) AllDIGIT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDIGIT)
}

func (s *MinuteContext) DIGIT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, i)
}

func (s *MinuteContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *MinuteContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *MinuteContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterMinute(s)
	}
}

func (s *MinuteContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitMinute(s)
	}
}

func (p *RFC5424Parser) Minute() (localctx IMinuteContext) {
	localctx = NewMinuteContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 22, RFC5424ParserRULE_minute)

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
		p.SetState(161)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(162)
		p.Match(RFC5424ParserDIGIT)
	}

	return localctx
}

// ISecondContext is an interface to support dynamic dispatch.
type ISecondContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsSecondContext differentiates from other interfaces.
	IsSecondContext()
}

type SecondContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptySecondContext() *SecondContext {
	var p = new(SecondContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_second
	return p
}

func (*SecondContext) IsSecondContext() {}

func NewSecondContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SecondContext {
	var p = new(SecondContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_second

	return p
}

func (s *SecondContext) GetParser() antlr.Parser { return s.parser }

func (s *SecondContext) AllDIGIT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDIGIT)
}

func (s *SecondContext) DIGIT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, i)
}

func (s *SecondContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SecondContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *SecondContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterSecond(s)
	}
}

func (s *SecondContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitSecond(s)
	}
}

func (p *RFC5424Parser) Second() (localctx ISecondContext) {
	localctx = NewSecondContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 24, RFC5424ParserRULE_second)

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
		p.SetState(164)
		p.Match(RFC5424ParserDIGIT)
	}
	{
		p.SetState(165)
		p.Match(RFC5424ParserDIGIT)
	}

	return localctx
}

// INanoContext is an interface to support dynamic dispatch.
type INanoContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsNanoContext differentiates from other interfaces.
	IsNanoContext()
}

type NanoContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyNanoContext() *NanoContext {
	var p = new(NanoContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_nano
	return p
}

func (*NanoContext) IsNanoContext() {}

func NewNanoContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *NanoContext {
	var p = new(NanoContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_nano

	return p
}

func (s *NanoContext) GetParser() antlr.Parser { return s.parser }

func (s *NanoContext) AllDIGIT() []antlr.TerminalNode {
	return s.GetTokens(RFC5424ParserDIGIT)
}

func (s *NanoContext) DIGIT(i int) antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, i)
}

func (s *NanoContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *NanoContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *NanoContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterNano(s)
	}
}

func (s *NanoContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitNano(s)
	}
}

func (p *RFC5424Parser) Nano() (localctx INanoContext) {
	localctx = NewNanoContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 26, RFC5424ParserRULE_nano)
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
	p.SetState(168)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = _la == RFC5424ParserDIGIT {
		{
			p.SetState(167)
			p.Match(RFC5424ParserDIGIT)
		}

		p.SetState(170)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
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

func (s *TimezoneContext) Timezonenum() ITimezonenumContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*ITimezonenumContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(ITimezonenumContext)
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
	p.EnterRule(localctx, 28, RFC5424ParserRULE_timezone)

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

	p.SetState(174)
	p.GetErrorHandler().Sync(p)

	switch p.GetTokenStream().LA(1) {
	case RFC5424ParserASCII:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(172)
			p.Match(RFC5424ParserASCII)
		}

	case RFC5424ParserPLUS, RFC5424ParserHYPHEN:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(173)
			p.Timezonenum()
		}

	default:
		panic(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
	}

	return localctx
}

// ITimezonenumContext is an interface to support dynamic dispatch.
type ITimezonenumContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsTimezonenumContext differentiates from other interfaces.
	IsTimezonenumContext()
}

type TimezonenumContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyTimezonenumContext() *TimezonenumContext {
	var p = new(TimezonenumContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_timezonenum
	return p
}

func (*TimezonenumContext) IsTimezonenumContext() {}

func NewTimezonenumContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *TimezonenumContext {
	var p = new(TimezonenumContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_timezonenum

	return p
}

func (s *TimezonenumContext) GetParser() antlr.Parser { return s.parser }

func (s *TimezonenumContext) Hour() IHourContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IHourContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IHourContext)
}

func (s *TimezonenumContext) COLON() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCOLON, 0)
}

func (s *TimezonenumContext) Minute() IMinuteContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IMinuteContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IMinuteContext)
}

func (s *TimezonenumContext) PLUS() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserPLUS, 0)
}

func (s *TimezonenumContext) HYPHEN() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserHYPHEN, 0)
}

func (s *TimezonenumContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *TimezonenumContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *TimezonenumContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterTimezonenum(s)
	}
}

func (s *TimezonenumContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitTimezonenum(s)
	}
}

func (p *RFC5424Parser) Timezonenum() (localctx ITimezonenumContext) {
	localctx = NewTimezonenumContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 30, RFC5424ParserRULE_timezonenum)
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
		p.SetState(176)
		_la = p.GetTokenStream().LA(1)

		if !(_la == RFC5424ParserPLUS || _la == RFC5424ParserHYPHEN) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}
	{
		p.SetState(177)
		p.Hour()
	}
	{
		p.SetState(178)
		p.Match(RFC5424ParserCOLON)
	}
	{
		p.SetState(179)
		p.Minute()
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

func (s *PriContext) OPENBRACKET() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserOPENBRACKET, 0)
}

func (s *PriContext) CLOSEBRACKET() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCLOSEBRACKET, 0)
}

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
	p.EnterRule(localctx, 32, RFC5424ParserRULE_pri)
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
		p.SetState(181)
		p.Match(RFC5424ParserOPENBRACKET)
	}
	p.SetState(183)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = _la == RFC5424ParserDIGIT {
		{
			p.SetState(182)
			p.Match(RFC5424ParserDIGIT)
		}

		p.SetState(185)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(187)
		p.Match(RFC5424ParserCLOSEBRACKET)
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

func (s *VersionContext) DIGIT() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, 0)
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
	p.EnterRule(localctx, 34, RFC5424ParserRULE_version)

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
		p.SetState(189)
		p.Match(RFC5424ParserDIGIT)
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

func (s *HostnameContext) HYPHEN() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserHYPHEN, 0)
}

func (s *HostnameContext) AllAllascii() []IAllasciiContext {
	var ts = s.GetTypedRuleContexts(reflect.TypeOf((*IAllasciiContext)(nil)).Elem())
	var tst = make([]IAllasciiContext, len(ts))

	for i, t := range ts {
		if t != nil {
			tst[i] = t.(IAllasciiContext)
		}
	}

	return tst
}

func (s *HostnameContext) Allascii(i int) IAllasciiContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IAllasciiContext)(nil)).Elem(), i)

	if t == nil {
		return nil
	}

	return t.(IAllasciiContext)
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
	p.EnterRule(localctx, 36, RFC5424ParserRULE_hostname)
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

	p.SetState(198)
	p.GetErrorHandler().Sync(p)
	switch p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 15, p.GetParserRuleContext()) {
	case 1:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(191)
			p.Match(RFC5424ParserHYPHEN)
		}

	case 2:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(192)
			p.Allascii()
		}
		p.SetState(194)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)

		for ok := true; ok; ok = (((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<RFC5424ParserDIGIT)|(1<<RFC5424ParserDOT)|(1<<RFC5424ParserOPENBRACKET)|(1<<RFC5424ParserCLOSEBRACKET)|(1<<RFC5424ParserPLUS)|(1<<RFC5424ParserHYPHEN)|(1<<RFC5424ParserCOLON)|(1<<RFC5424ParserEQUAL)|(1<<RFC5424ParserQUOTE)|(1<<RFC5424ParserOPENSQUARE)|(1<<RFC5424ParserCLOSESQUARE)|(1<<RFC5424ParserANTISLASH)|(1<<RFC5424ParserASCII))) != 0) {
			{
				p.SetState(193)
				p.Allascii()
			}

			p.SetState(196)
			p.GetErrorHandler().Sync(p)
			_la = p.GetTokenStream().LA(1)
		}

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

func (s *AppnameContext) HYPHEN() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserHYPHEN, 0)
}

func (s *AppnameContext) AllAllascii() []IAllasciiContext {
	var ts = s.GetTypedRuleContexts(reflect.TypeOf((*IAllasciiContext)(nil)).Elem())
	var tst = make([]IAllasciiContext, len(ts))

	for i, t := range ts {
		if t != nil {
			tst[i] = t.(IAllasciiContext)
		}
	}

	return tst
}

func (s *AppnameContext) Allascii(i int) IAllasciiContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IAllasciiContext)(nil)).Elem(), i)

	if t == nil {
		return nil
	}

	return t.(IAllasciiContext)
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
	p.EnterRule(localctx, 38, RFC5424ParserRULE_appname)
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

	p.SetState(207)
	p.GetErrorHandler().Sync(p)
	switch p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 17, p.GetParserRuleContext()) {
	case 1:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(200)
			p.Match(RFC5424ParserHYPHEN)
		}

	case 2:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(201)
			p.Allascii()
		}
		p.SetState(203)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)

		for ok := true; ok; ok = (((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<RFC5424ParserDIGIT)|(1<<RFC5424ParserDOT)|(1<<RFC5424ParserOPENBRACKET)|(1<<RFC5424ParserCLOSEBRACKET)|(1<<RFC5424ParserPLUS)|(1<<RFC5424ParserHYPHEN)|(1<<RFC5424ParserCOLON)|(1<<RFC5424ParserEQUAL)|(1<<RFC5424ParserQUOTE)|(1<<RFC5424ParserOPENSQUARE)|(1<<RFC5424ParserCLOSESQUARE)|(1<<RFC5424ParserANTISLASH)|(1<<RFC5424ParserASCII))) != 0) {
			{
				p.SetState(202)
				p.Allascii()
			}

			p.SetState(205)
			p.GetErrorHandler().Sync(p)
			_la = p.GetTokenStream().LA(1)
		}

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

func (s *MsgidContext) HYPHEN() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserHYPHEN, 0)
}

func (s *MsgidContext) AllAllascii() []IAllasciiContext {
	var ts = s.GetTypedRuleContexts(reflect.TypeOf((*IAllasciiContext)(nil)).Elem())
	var tst = make([]IAllasciiContext, len(ts))

	for i, t := range ts {
		if t != nil {
			tst[i] = t.(IAllasciiContext)
		}
	}

	return tst
}

func (s *MsgidContext) Allascii(i int) IAllasciiContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IAllasciiContext)(nil)).Elem(), i)

	if t == nil {
		return nil
	}

	return t.(IAllasciiContext)
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
	p.EnterRule(localctx, 40, RFC5424ParserRULE_msgid)
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

	p.SetState(216)
	p.GetErrorHandler().Sync(p)
	switch p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 19, p.GetParserRuleContext()) {
	case 1:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(209)
			p.Match(RFC5424ParserHYPHEN)
		}

	case 2:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(210)
			p.Allascii()
		}
		p.SetState(212)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)

		for ok := true; ok; ok = (((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<RFC5424ParserDIGIT)|(1<<RFC5424ParserDOT)|(1<<RFC5424ParserOPENBRACKET)|(1<<RFC5424ParserCLOSEBRACKET)|(1<<RFC5424ParserPLUS)|(1<<RFC5424ParserHYPHEN)|(1<<RFC5424ParserCOLON)|(1<<RFC5424ParserEQUAL)|(1<<RFC5424ParserQUOTE)|(1<<RFC5424ParserOPENSQUARE)|(1<<RFC5424ParserCLOSESQUARE)|(1<<RFC5424ParserANTISLASH)|(1<<RFC5424ParserASCII))) != 0) {
			{
				p.SetState(211)
				p.Allascii()
			}

			p.SetState(214)
			p.GetErrorHandler().Sync(p)
			_la = p.GetTokenStream().LA(1)
		}

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

func (s *ProcidContext) HYPHEN() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserHYPHEN, 0)
}

func (s *ProcidContext) AllAllascii() []IAllasciiContext {
	var ts = s.GetTypedRuleContexts(reflect.TypeOf((*IAllasciiContext)(nil)).Elem())
	var tst = make([]IAllasciiContext, len(ts))

	for i, t := range ts {
		if t != nil {
			tst[i] = t.(IAllasciiContext)
		}
	}

	return tst
}

func (s *ProcidContext) Allascii(i int) IAllasciiContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IAllasciiContext)(nil)).Elem(), i)

	if t == nil {
		return nil
	}

	return t.(IAllasciiContext)
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
	p.EnterRule(localctx, 42, RFC5424ParserRULE_procid)
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

	p.SetState(225)
	p.GetErrorHandler().Sync(p)
	switch p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 21, p.GetParserRuleContext()) {
	case 1:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(218)
			p.Match(RFC5424ParserHYPHEN)
		}

	case 2:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(219)
			p.Allascii()
		}
		p.SetState(221)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)

		for ok := true; ok; ok = (((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<RFC5424ParserDIGIT)|(1<<RFC5424ParserDOT)|(1<<RFC5424ParserOPENBRACKET)|(1<<RFC5424ParserCLOSEBRACKET)|(1<<RFC5424ParserPLUS)|(1<<RFC5424ParserHYPHEN)|(1<<RFC5424ParserCOLON)|(1<<RFC5424ParserEQUAL)|(1<<RFC5424ParserQUOTE)|(1<<RFC5424ParserOPENSQUARE)|(1<<RFC5424ParserCLOSESQUARE)|(1<<RFC5424ParserANTISLASH)|(1<<RFC5424ParserASCII))) != 0) {
			{
				p.SetState(220)
				p.Allascii()
			}

			p.SetState(223)
			p.GetErrorHandler().Sync(p)
			_la = p.GetTokenStream().LA(1)
		}

	}

	return localctx
}

// IAllasciiContext is an interface to support dynamic dispatch.
type IAllasciiContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsAllasciiContext differentiates from other interfaces.
	IsAllasciiContext()
}

type AllasciiContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyAllasciiContext() *AllasciiContext {
	var p = new(AllasciiContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_allascii
	return p
}

func (*AllasciiContext) IsAllasciiContext() {}

func NewAllasciiContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *AllasciiContext {
	var p = new(AllasciiContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_allascii

	return p
}

func (s *AllasciiContext) GetParser() antlr.Parser { return s.parser }

func (s *AllasciiContext) ASCII() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserASCII, 0)
}

func (s *AllasciiContext) Specialname() ISpecialnameContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*ISpecialnameContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(ISpecialnameContext)
}

func (s *AllasciiContext) EQUAL() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserEQUAL, 0)
}

func (s *AllasciiContext) CLOSESQUARE() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCLOSESQUARE, 0)
}

func (s *AllasciiContext) QUOTE() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserQUOTE, 0)
}

func (s *AllasciiContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *AllasciiContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *AllasciiContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterAllascii(s)
	}
}

func (s *AllasciiContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitAllascii(s)
	}
}

func (p *RFC5424Parser) Allascii() (localctx IAllasciiContext) {
	localctx = NewAllasciiContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 44, RFC5424ParserRULE_allascii)

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

	p.SetState(232)
	p.GetErrorHandler().Sync(p)

	switch p.GetTokenStream().LA(1) {
	case RFC5424ParserASCII:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(227)
			p.Match(RFC5424ParserASCII)
		}

	case RFC5424ParserDIGIT, RFC5424ParserDOT, RFC5424ParserOPENBRACKET, RFC5424ParserCLOSEBRACKET, RFC5424ParserPLUS, RFC5424ParserHYPHEN, RFC5424ParserCOLON, RFC5424ParserOPENSQUARE, RFC5424ParserANTISLASH:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(228)
			p.Specialname()
		}

	case RFC5424ParserEQUAL:
		p.EnterOuterAlt(localctx, 3)
		{
			p.SetState(229)
			p.Match(RFC5424ParserEQUAL)
		}

	case RFC5424ParserCLOSESQUARE:
		p.EnterOuterAlt(localctx, 4)
		{
			p.SetState(230)
			p.Match(RFC5424ParserCLOSESQUARE)
		}

	case RFC5424ParserQUOTE:
		p.EnterOuterAlt(localctx, 5)
		{
			p.SetState(231)
			p.Match(RFC5424ParserQUOTE)
		}

	default:
		panic(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
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
	p.EnterRule(localctx, 46, RFC5424ParserRULE_structured)
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

	p.SetState(240)
	p.GetErrorHandler().Sync(p)

	switch p.GetTokenStream().LA(1) {
	case RFC5424ParserHYPHEN:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(234)
			p.Match(RFC5424ParserHYPHEN)
		}

	case RFC5424ParserOPENSQUARE:
		p.EnterOuterAlt(localctx, 2)
		p.SetState(236)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)

		for ok := true; ok; ok = _la == RFC5424ParserOPENSQUARE {
			{
				p.SetState(235)
				p.Element()
			}

			p.SetState(238)
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
	p.EnterRule(localctx, 48, RFC5424ParserRULE_element)
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
		p.SetState(242)
		p.Match(RFC5424ParserOPENSQUARE)
	}
	{
		p.SetState(243)
		p.Sid()
	}
	p.SetState(248)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for _la == RFC5424ParserSP {
		{
			p.SetState(244)
			p.Match(RFC5424ParserSP)
		}
		{
			p.SetState(245)
			p.Param()
		}

		p.SetState(250)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(251)
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
	p.EnterRule(localctx, 50, RFC5424ParserRULE_sid)

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
		p.SetState(253)
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
	p.EnterRule(localctx, 52, RFC5424ParserRULE_param)

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
		p.SetState(255)
		p.Name()
	}
	{
		p.SetState(256)
		p.Match(RFC5424ParserEQUAL)
	}
	{
		p.SetState(257)
		p.Match(RFC5424ParserQUOTE)
	}
	{
		p.SetState(258)
		p.Value()
	}
	{
		p.SetState(259)
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

func (s *NameContext) AllSpecialname() []ISpecialnameContext {
	var ts = s.GetTypedRuleContexts(reflect.TypeOf((*ISpecialnameContext)(nil)).Elem())
	var tst = make([]ISpecialnameContext, len(ts))

	for i, t := range ts {
		if t != nil {
			tst[i] = t.(ISpecialnameContext)
		}
	}

	return tst
}

func (s *NameContext) Specialname(i int) ISpecialnameContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*ISpecialnameContext)(nil)).Elem(), i)

	if t == nil {
		return nil
	}

	return t.(ISpecialnameContext)
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
	p.EnterRule(localctx, 54, RFC5424ParserRULE_name)
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
	p.SetState(263)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ok := true; ok; ok = (((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<RFC5424ParserDIGIT)|(1<<RFC5424ParserDOT)|(1<<RFC5424ParserOPENBRACKET)|(1<<RFC5424ParserCLOSEBRACKET)|(1<<RFC5424ParserPLUS)|(1<<RFC5424ParserHYPHEN)|(1<<RFC5424ParserCOLON)|(1<<RFC5424ParserOPENSQUARE)|(1<<RFC5424ParserANTISLASH)|(1<<RFC5424ParserASCII))) != 0) {
		p.SetState(263)
		p.GetErrorHandler().Sync(p)

		switch p.GetTokenStream().LA(1) {
		case RFC5424ParserASCII:
			{
				p.SetState(261)
				p.Match(RFC5424ParserASCII)
			}

		case RFC5424ParserDIGIT, RFC5424ParserDOT, RFC5424ParserOPENBRACKET, RFC5424ParserCLOSEBRACKET, RFC5424ParserPLUS, RFC5424ParserHYPHEN, RFC5424ParserCOLON, RFC5424ParserOPENSQUARE, RFC5424ParserANTISLASH:
			{
				p.SetState(262)
				p.Specialname()
			}

		default:
			panic(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
		}

		p.SetState(265)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}

	return localctx
}

// ISpecialnameContext is an interface to support dynamic dispatch.
type ISpecialnameContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsSpecialnameContext differentiates from other interfaces.
	IsSpecialnameContext()
}

type SpecialnameContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptySpecialnameContext() *SpecialnameContext {
	var p = new(SpecialnameContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_specialname
	return p
}

func (*SpecialnameContext) IsSpecialnameContext() {}

func NewSpecialnameContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SpecialnameContext {
	var p = new(SpecialnameContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_specialname

	return p
}

func (s *SpecialnameContext) GetParser() antlr.Parser { return s.parser }

func (s *SpecialnameContext) DIGIT() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, 0)
}

func (s *SpecialnameContext) DOT() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDOT, 0)
}

func (s *SpecialnameContext) OPENBRACKET() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserOPENBRACKET, 0)
}

func (s *SpecialnameContext) CLOSEBRACKET() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCLOSEBRACKET, 0)
}

func (s *SpecialnameContext) PLUS() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserPLUS, 0)
}

func (s *SpecialnameContext) HYPHEN() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserHYPHEN, 0)
}

func (s *SpecialnameContext) COLON() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCOLON, 0)
}

func (s *SpecialnameContext) OPENSQUARE() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserOPENSQUARE, 0)
}

func (s *SpecialnameContext) ANTISLASH() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserANTISLASH, 0)
}

func (s *SpecialnameContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SpecialnameContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *SpecialnameContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterSpecialname(s)
	}
}

func (s *SpecialnameContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitSpecialname(s)
	}
}

func (p *RFC5424Parser) Specialname() (localctx ISpecialnameContext) {
	localctx = NewSpecialnameContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 56, RFC5424ParserRULE_specialname)
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
		p.SetState(267)
		_la = p.GetTokenStream().LA(1)

		if !(((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<RFC5424ParserDIGIT)|(1<<RFC5424ParserDOT)|(1<<RFC5424ParserOPENBRACKET)|(1<<RFC5424ParserCLOSEBRACKET)|(1<<RFC5424ParserPLUS)|(1<<RFC5424ParserHYPHEN)|(1<<RFC5424ParserCOLON)|(1<<RFC5424ParserOPENSQUARE)|(1<<RFC5424ParserANTISLASH))) != 0) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
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

func (s *ValueContext) AllSpecialvalue() []ISpecialvalueContext {
	var ts = s.GetTypedRuleContexts(reflect.TypeOf((*ISpecialvalueContext)(nil)).Elem())
	var tst = make([]ISpecialvalueContext, len(ts))

	for i, t := range ts {
		if t != nil {
			tst[i] = t.(ISpecialvalueContext)
		}
	}

	return tst
}

func (s *ValueContext) Specialvalue(i int) ISpecialvalueContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*ISpecialvalueContext)(nil)).Elem(), i)

	if t == nil {
		return nil
	}

	return t.(ISpecialvalueContext)
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
	p.EnterRule(localctx, 58, RFC5424ParserRULE_value)
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
	p.SetState(280)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for ((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<RFC5424ParserDIGIT)|(1<<RFC5424ParserDOT)|(1<<RFC5424ParserOPENBRACKET)|(1<<RFC5424ParserCLOSEBRACKET)|(1<<RFC5424ParserPLUS)|(1<<RFC5424ParserHYPHEN)|(1<<RFC5424ParserCOLON)|(1<<RFC5424ParserSP)|(1<<RFC5424ParserEQUAL)|(1<<RFC5424ParserOPENSQUARE)|(1<<RFC5424ParserANTISLASH)|(1<<RFC5424ParserASCII)|(1<<RFC5424ParserVALUECHAR))) != 0 {
		p.SetState(278)
		p.GetErrorHandler().Sync(p)
		switch p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 28, p.GetParserRuleContext()) {
		case 1:
			{
				p.SetState(269)
				p.Match(RFC5424ParserASCII)
			}

		case 2:
			{
				p.SetState(270)
				p.Match(RFC5424ParserVALUECHAR)
			}

		case 3:
			{
				p.SetState(271)
				p.Specialvalue()
			}

		case 4:
			{
				p.SetState(272)
				p.Match(RFC5424ParserANTISLASH)
			}
			{
				p.SetState(273)
				p.Match(RFC5424ParserANTISLASH)
			}

		case 5:
			{
				p.SetState(274)
				p.Match(RFC5424ParserANTISLASH)
			}
			{
				p.SetState(275)
				p.Match(RFC5424ParserCLOSESQUARE)
			}

		case 6:
			{
				p.SetState(276)
				p.Match(RFC5424ParserANTISLASH)
			}
			{
				p.SetState(277)
				p.Match(RFC5424ParserQUOTE)
			}

		}

		p.SetState(282)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}

	return localctx
}

// ISpecialvalueContext is an interface to support dynamic dispatch.
type ISpecialvalueContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsSpecialvalueContext differentiates from other interfaces.
	IsSpecialvalueContext()
}

type SpecialvalueContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptySpecialvalueContext() *SpecialvalueContext {
	var p = new(SpecialvalueContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = RFC5424ParserRULE_specialvalue
	return p
}

func (*SpecialvalueContext) IsSpecialvalueContext() {}

func NewSpecialvalueContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SpecialvalueContext {
	var p = new(SpecialvalueContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = RFC5424ParserRULE_specialvalue

	return p
}

func (s *SpecialvalueContext) GetParser() antlr.Parser { return s.parser }

func (s *SpecialvalueContext) DIGIT() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDIGIT, 0)
}

func (s *SpecialvalueContext) DOT() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserDOT, 0)
}

func (s *SpecialvalueContext) OPENBRACKET() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserOPENBRACKET, 0)
}

func (s *SpecialvalueContext) CLOSEBRACKET() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCLOSEBRACKET, 0)
}

func (s *SpecialvalueContext) PLUS() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserPLUS, 0)
}

func (s *SpecialvalueContext) HYPHEN() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserHYPHEN, 0)
}

func (s *SpecialvalueContext) COLON() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserCOLON, 0)
}

func (s *SpecialvalueContext) SP() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserSP, 0)
}

func (s *SpecialvalueContext) EQUAL() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserEQUAL, 0)
}

func (s *SpecialvalueContext) OPENSQUARE() antlr.TerminalNode {
	return s.GetToken(RFC5424ParserOPENSQUARE, 0)
}

func (s *SpecialvalueContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SpecialvalueContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *SpecialvalueContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.EnterSpecialvalue(s)
	}
}

func (s *SpecialvalueContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(RFC5424Listener); ok {
		listenerT.ExitSpecialvalue(s)
	}
}

func (p *RFC5424Parser) Specialvalue() (localctx ISpecialvalueContext) {
	localctx = NewSpecialvalueContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 60, RFC5424ParserRULE_specialvalue)
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
		p.SetState(283)
		_la = p.GetTokenStream().LA(1)

		if !(((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<RFC5424ParserDIGIT)|(1<<RFC5424ParserDOT)|(1<<RFC5424ParserOPENBRACKET)|(1<<RFC5424ParserCLOSEBRACKET)|(1<<RFC5424ParserPLUS)|(1<<RFC5424ParserHYPHEN)|(1<<RFC5424ParserCOLON)|(1<<RFC5424ParserSP)|(1<<RFC5424ParserEQUAL)|(1<<RFC5424ParserOPENSQUARE))) != 0) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

	return localctx
}
