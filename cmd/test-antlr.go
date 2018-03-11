// Copyright © 2018 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"os"

	"github.com/antlr/antlr4/runtime/Go/antlr"
	"github.com/spf13/cobra"
	"github.com/stephane-martin/skewer/grammars/rfc5424"
)

type ErrorStrategy struct {
	*antlr.DefaultErrorStrategy
}

func NewErrorStrategy() *ErrorStrategy {
	return &ErrorStrategy{
		DefaultErrorStrategy: antlr.NewDefaultErrorStrategy(),
	}
}

func (s *ErrorStrategy) Sync(antlr.Parser) {

}

type ErrorListener struct {
	*antlr.DefaultErrorListener
}

func NewErrorListener() *ErrorListener {
	return &ErrorListener{
		DefaultErrorListener: antlr.NewDefaultErrorListener(),
	}
}

func (el *ErrorListener) SyntaxError(rec antlr.Recognizer, offendingSymbol interface{}, line, column int, msg string, e antlr.RecognitionException) {
	fmt.Fprintf(os.Stderr, "*** %s ***\n", msg)
}

type Listener struct {
	*rfc5424.BaseRFC5424Listener
}

func NewListener() *Listener {
	return &Listener{
		BaseRFC5424Listener: &rfc5424.BaseRFC5424Listener{},
	}
}

func (l *Listener) ExitAppname(ctx *rfc5424.AppnameContext) {
	fmt.Println("PLOP")
	fmt.Println(ctx.GetText())
}

func (l *Listener) ExitStructured(ctx *rfc5424.StructuredContext) {
	fmt.Println("PLUP")
	fmt.Println(ctx.GetText())
}

// testAntlrCmd represents the testAntlr command
var testAntlrCmd = &cobra.Command{
	Use:   "test-antlr",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("test-antlr called")
		line := os.Args[2]

		is := antlr.NewInputStream(line)
		lexer := rfc5424.NewRFC5424Lexer(is)
		for {
			t := lexer.NextToken()
			if t.GetTokenType() == antlr.TokenEOF {
				break
			}
			fmt.Printf("%s (%q)\n", lexer.SymbolicNames[t.GetTokenType()], t.GetText())
		}

		is = antlr.NewInputStream(line)
		lexer = rfc5424.NewRFC5424Lexer(is)
		stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
		parser := rfc5424.NewRFC5424Parser(stream)
		parser.RemoveErrorListeners()
		parser.AddErrorListener(NewErrorListener())
		parser.AddErrorListener(antlr.NewDiagnosticErrorListener(false))
		parser.SetErrorHandler(NewErrorStrategy())
		parser.BuildParseTrees = true
		parser.GetInterpreter().SetPredictionMode(antlr.PredictionModeSLL)
		tree := parser.Full()
		antlr.ParseTreeWalkerDefault.Walk(NewListener(), tree)

	},
}

func init() {
	RootCmd.AddCommand(testAntlrCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// testAntlrCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// testAntlrCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
