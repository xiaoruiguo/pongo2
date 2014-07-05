package pongo2

import (
	"errors"
	"fmt"
	"strings"
)

type IFilteredEvaluator interface {
	IEvaluator
}

type IEvaluator interface {
	Evaluate(*ExecutionContext) (*Value, error)
	FilterApplied(name string) bool
}

type INode interface {
	Execute(*ExecutionContext) (string, error)
}

type INodeEvaluator interface {
	INode
	IEvaluator
}

// The parser provides you a comprehensive and easy tool to
// work with the template document and arguments provided by
// the user for your custom tag.
//
// The parser works on a token list which will be provided by pongo2.
// A token is a unit you can work with. Tokens are either of type identifier,
// string, number, keyword, HTML or symbol.
//
// (See Token's documentation for more about tokens)
type Parser struct {
	name   string
	idx    int
	tokens []*Token

	// if the parser parses a template document, here will be
	// a reference to it (needed to access the template through Tags)
	template *Template
}

// Creates a new parser to parse tokens.
// Used inside pongo2 to parse documents and to provide an easy-to-use
// parser for tag authors
func newParser(name string, tokens []*Token, template *Template) *Parser {
	return &Parser{
		name:     name,
		tokens:   tokens,
		template: template,
	}
}

// Consume one token. It will be gone forever.
func (p *Parser) Consume() {
	p.ConsumeN(1)
}

// Consume N tokens. They will be gone forever.
func (p *Parser) ConsumeN(count int) {
	p.idx += count
}

// Returns the current token.
func (p *Parser) Current() *Token {
	return p.Get(p.idx)
}

// Returns the CURRENT token if the given type matches.
// Consumes this token on success.
func (p *Parser) MatchType(typ TokenType) *Token {
	if t := p.PeekType(typ); t != nil {
		p.Consume()
		return t
	}
	return nil
}

// Returns the CURRENT token if the given type AND value matches.
// Consumes this token on success.
func (p *Parser) Match(typ TokenType, val string) *Token {
	if t := p.Peek(typ, val); t != nil {
		p.Consume()
		return t
	}
	return nil
}

// Returns the CURRENT token if the given type AND *one* of
// the given values matches.
// Consumes this token on success.
func (p *Parser) MatchOne(typ TokenType, vals ...string) *Token {
	for _, val := range vals {
		if t := p.Peek(typ, val); t != nil {
			p.Consume()
			return t
		}
	}
	return nil
}

// Returns the CURRENT token if the given type matches.
// It DOES NOT consume the token.
func (p *Parser) PeekType(typ TokenType) *Token {
	return p.PeekTypeN(0, typ)
}

// Returns the CURRENT token if the given type AND value matches.
// It DOES NOT consume the token.
func (p *Parser) Peek(typ TokenType, val string) *Token {
	return p.PeekN(0, typ, val)
}

// Returns the CURRENT token if the given type AND *one* of
// the given values matches.
// It DOES NOT consume the token.
func (p *Parser) PeekOne(typ TokenType, vals ...string) *Token {
	for _, v := range vals {
		t := p.PeekN(0, typ, v)
		if t != nil {
			return t
		}
	}
	return nil
}

// Returns the tokens[current position + shift] token if the
// given type AND value matches for that token.
// DOES NOT consume the token.
func (p *Parser) PeekN(shift int, typ TokenType, val string) *Token {
	t := p.Get(p.idx + shift)
	if t != nil {
		if t.Typ == typ && t.Val == val {
			return t
		}
	}
	return nil
}

// Returns the tokens[current position + shift] token if the given type matches.
// DOES NOT consume the token for that token.
func (p *Parser) PeekTypeN(shift int, typ TokenType) *Token {
	t := p.Get(p.idx + shift)
	if t != nil {
		if t.Typ == typ {
			return t
		}
	}
	return nil
}

// Returns the UNCONSUMED token count.
func (p *Parser) Remaining() int {
	return len(p.tokens) - p.idx
}

// Returns the total token count.
func (p *Parser) Count() int {
	return len(p.tokens)
}

// Returns tokens[i] or NIL (if i >= len(tokens))
func (p *Parser) Get(i int) *Token {
	if i < len(p.tokens) {
		return p.tokens[i]
	}
	return nil
}

// Returns tokens[current-position + shift] or NIL
// (if (current-position + i) >= len(tokens))
func (p *Parser) GetR(shift int) *Token {
	i := p.idx + shift
	return p.Get(i)
}

// Produces a nice error message and returns an error-object.
// The 'token'-argument is optional. If provided, it will take
// the token's position information. If not provided, it will
// automatically use the CURRENT token's position information.
func (p *Parser) Error(msg string, token *Token) error {
	if token == nil {
		// Set current token
		token = p.Current()
		if token == nil {
			// Set to last token
			if len(p.tokens) > 0 {
				token = p.tokens[len(p.tokens)-1]
			}
		}
	}
	pos := ""
	if token != nil {
		// No tokens available
		// TODO: Add location (from where?)
		pos = fmt.Sprintf(" | Line %d Col %d (%s)",
			token.Line, token.Col, token.String())
	}
	return errors.New(
		fmt.Sprintf("[Parse Error in %s%s] %s",
			p.name, pos, msg,
		))
}

// Wraps all nodes between starting tag and "{% endtag %}" and provides
// one simple interface to execute the wrapped nodes
func (p *Parser) WrapUntilTag(names ...string) (*NodeWrapper, error) {
	wrapper := &NodeWrapper{}

	for p.Remaining() > 0 {
		// New tag, check whether we have to stop wrapping here
		if p.Peek(TokenSymbol, "{%") != nil {
			tag_ident := p.PeekTypeN(1, TokenIdentifier)

			if tag_ident != nil {
				// We've found a (!) end-tag

				found := false
				name := ""
				for _, n := range names {
					if tag_ident.Val == n {
						name = n
						found = true
						break
					}
				}

				// We only process the tag if we've found an end tag
				if found {
					if p.PeekN(2, TokenSymbol, "%}") != nil {
						// Okay, end the wrapping here
						wrapper.Endtag = tag_ident.Val

						p.ConsumeN(3)
						return wrapper, nil
					} else {
						// Arguments provided, which is not allowed
						return nil, p.Error(fmt.Sprintf("No arguments allowed for tag '%s'", name), tag_ident)
					}
				}
				/* else {
					// unexpected EOF
					return nil, p.Error(fmt.Sprintf("Unexpected EOF, tag '%s' not closed", name), tpl.tokens[tpl.parser_idx+1])
				}*/
			}

		}
		/*else {
			// unexpected EOF
			return nil, p.Error(fmt.Sprintf("Unexpected EOF (expected end-tags '%s')", strings.Join(names, ", ")), t)
		}*/

		// Otherwise process next element to be wrapped
		node, err := p.parseDocElement()
		if err != nil {
			return nil, err
		}
		wrapper.nodes = append(wrapper.nodes, node)
	}

	return nil, p.Error(fmt.Sprintf("Unexpected EOF, expected tag %s.", strings.Join(names, " or ")), nil)
}
