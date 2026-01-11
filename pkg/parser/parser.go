// Package parser provides PSIL parsing using Participle v2.
// Grammar is defined as Go structs with tags.
package parser

import (
	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
	"github.com/psilLang/psil/pkg/types"
)

// AST Node types - parsed from source, converted to types.Value for execution

// Program is the top-level AST node
type Program struct {
	Statements []*Statement `@@*`
}

// Statement is either a definition or an expression
type Statement struct {
	Definition *Definition `  @@`
	Expression *Expression `| @@`
}

// Definition: DEFINE name == quotation .
type Definition struct {
	Name string     `"DEFINE" @Ident "==" `
	Body *Quotation `@@ "."`
}

// Quotation: [ expr* ]
type Quotation struct {
	Items []*Expression `"[" @@* "]"`
}

// Expression: literal | symbol | quotation
type Expression struct {
	Number    *float64   `  @Number`
	String    *string    `| @String`
	Boolean   *string    `| @("true" | "false")`
	Symbol    *string    `| @Ident`
	Operator  *string    `| @Operator`
	Quotation *Quotation `| @@`
}

// PSIL lexer definition
var psilLexer = lexer.MustSimple([]lexer.SimpleRule{
	// Skip whitespace and comments
	{Name: "Whitespace", Pattern: `[\s]+`},
	{Name: "Comment", Pattern: `%[^\n]*`},

	// Keywords
	{Name: "DEFINE", Pattern: `DEFINE`},

	// Literals
	{Name: "Number", Pattern: `-?[0-9]+(\.[0-9]+)?`},
	{Name: "String", Pattern: `"[^"]*"`},

	// Operators (single char ops that are valid symbols)
	{Name: "Operator", Pattern: `[+\-*/<=>.!?@#$&|~^]+`},

	// Brackets and punctuation
	{Name: "Punct", Pattern: `[\[\]==]`},

	// Identifiers (including keywords like true, false, dup, swap, img-new, etc.)
	// Allow hyphens in identifiers for names like img-new, img-save
	{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_-]*`},
})

// Parser is the PSIL parser
var Parser = participle.MustBuild[Program](
	participle.Lexer(psilLexer),
	participle.Elide("Whitespace", "Comment"),
	participle.UseLookahead(2),
)

// Parse parses PSIL source code into a Program AST
func Parse(source string) (*Program, error) {
	return Parser.ParseString("", source)
}

// ParseFile parses a PSIL source file
func ParseFile(filename string) (*Program, error) {
	return Parser.ParseString(filename, "")
}

// ToValue converts an Expression AST node to a runtime Value
func (e *Expression) ToValue() types.Value {
	switch {
	case e.Number != nil:
		return types.Number(*e.Number)
	case e.String != nil:
		// Remove quotes from parsed string
		s := *e.String
		if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
			s = s[1 : len(s)-1]
		}
		return types.String(s)
	case e.Boolean != nil:
		return types.Boolean(*e.Boolean == "true")
	case e.Symbol != nil:
		return types.Symbol(*e.Symbol)
	case e.Operator != nil:
		return types.Symbol(*e.Operator)
	case e.Quotation != nil:
		return e.Quotation.ToValue()
	}
	return nil
}

// ToValue converts a Quotation AST node to a runtime Quotation
func (q *Quotation) ToValue() *types.Quotation {
	items := make([]types.Value, 0, len(q.Items))
	for _, item := range q.Items {
		if v := item.ToValue(); v != nil {
			items = append(items, v)
		}
	}
	return &types.Quotation{Items: items}
}

// ToValues converts a Program to a slice of Values for execution
func (p *Program) ToValues() ([]types.Value, map[string]*types.Quotation) {
	var values []types.Value
	definitions := make(map[string]*types.Quotation)

	for _, stmt := range p.Statements {
		if stmt.Definition != nil {
			// Store definition in the dictionary
			definitions[stmt.Definition.Name] = stmt.Definition.Body.ToValue()
		} else if stmt.Expression != nil {
			// Add expression to execution list
			if v := stmt.Expression.ToValue(); v != nil {
				values = append(values, v)
			}
		}
	}

	return values, definitions
}
