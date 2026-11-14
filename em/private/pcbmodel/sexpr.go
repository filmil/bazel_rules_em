// SPDX-License-Identifier: Apache-2.0

package pcbmodel

import (
	"fmt"
	"strconv"
	"strings"
)

// Node is an s-expression: a list (Kids), a bare symbol (Sym), a quoted
// string (Str, Quoted) or a number (Num, IsNum).
type Node struct {
	Kids   []*Node
	List   bool
	Sym    string
	Str    string
	Quoted bool
	Num    float64
	IsNum  bool
}

// Head is a list's first symbol, or "".
func (n *Node) Head() string {
	if n == nil || !n.List || len(n.Kids) == 0 || n.Kids[0].List || n.Kids[0].Quoted {
		return ""
	}
	return n.Kids[0].Sym
}

// All returns the children that are lists headed by name.
func (n *Node) All(name string) []*Node {
	var out []*Node
	if n == nil {
		return nil
	}
	for _, k := range n.Kids {
		if k.Head() == name {
			out = append(out, k)
		}
	}
	return out
}

// One returns the first child list headed by name, or nil.
func (n *Node) One(name string) *Node {
	if n == nil {
		return nil
	}
	for _, k := range n.Kids {
		if k.Head() == name {
			return k
		}
	}
	return nil
}

// Text is an atom's text: a symbol, a string, or a number as written.
func (n *Node) Text() string {
	switch {
	case n == nil:
		return ""
	case n.Quoted:
		return n.Str
	default:
		return n.Sym
	}
}

// Arg returns argument i (0 is the first after the head), or nil.
func (n *Node) Arg(i int) *Node {
	if n == nil || i+1 >= len(n.Kids) {
		return nil
	}
	return n.Kids[i+1]
}

// Float returns argument i as a number.
func (n *Node) Float(i int) (float64, error) {
	a := n.Arg(i)
	if a == nil || !a.IsNum {
		return 0, fmt.Errorf("(%s ...): argument %d is not a number", n.Head(), i+1)
	}
	return a.Num, nil
}

// Parse reads one s-expression, as KiCad writes them.
func Parse(text string) (*Node, error) {
	stack := []*Node{{List: true}}
	for i := 0; i < len(text); {
		c := text[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			i++
		case c == '(':
			stack = append(stack, &Node{List: true})
			i++
		case c == ')':
			if len(stack) == 1 {
				return nil, fmt.Errorf("an unbalanced ) at offset %d", i)
			}
			done := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			top := stack[len(stack)-1]
			top.Kids = append(top.Kids, done)
			i++
		case c == '"':
			var b strings.Builder
			j := i + 1
			for ; j < len(text) && text[j] != '"'; j++ {
				if text[j] == '\\' && j+1 < len(text) {
					j++
					switch text[j] {
					case 'n':
						b.WriteByte('\n')
					case 't':
						b.WriteByte('\t')
					default:
						b.WriteByte(text[j])
					}
					continue
				}
				b.WriteByte(text[j])
			}
			if j >= len(text) {
				return nil, fmt.Errorf("an unterminated string at offset %d", i)
			}
			top := stack[len(stack)-1]
			top.Kids = append(top.Kids, &Node{Str: b.String(), Quoted: true})
			i = j + 1
		default:
			j := i
			for j < len(text) && !strings.ContainsRune(" \t\r\n()\"", rune(text[j])) {
				j++
			}
			tok := text[i:j]
			n := &Node{Sym: tok}
			if v, err := strconv.ParseFloat(tok, 64); err == nil {
				n.Num, n.IsNum = v, true
			}
			top := stack[len(stack)-1]
			top.Kids = append(top.Kids, n)
			i = j
		}
	}
	if len(stack) != 1 || len(stack[0].Kids) != 1 {
		return nil, fmt.Errorf("unbalanced (: the text ends inside a list")
	}
	return stack[0].Kids[0], nil
}
