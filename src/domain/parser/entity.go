package parser

import (
	"fmt"
	"strings"
)

type GorutineGraph struct {
	Gorutines map[string]Goroutine
	Channels  map[string]Channel
	Edges     []Edge
}

type Goroutine struct {
	ID   string
	Func string
	File string
	TS   string
}
type Channel struct {
	Name string
	File string
	TS   string
}
type Edge struct {
	From  string
	To    string
	Label string
}

func (g *GorutineGraph) ToDot() string {
	var sb strings.Builder
	sb.WriteString("strict digraph goroutine_channels {\n")
	sb.WriteString("  // Nodes\n")

	for _, g := range g.Gorutines {
		sb.WriteString(fmt.Sprintf("  \"%s (ID: %s)\\n%s\\n%s\";\n", g.Func, g.ID, g.File, g.TS))
	}

	for _, ch := range g.Channels {
		label := fmt.Sprintf("channel %s", ch.Name)
		if closer, ok := g.getCloseBy(ch.Name); ok {
			label += fmt.Sprintf(" (closed by %s)", closer)
		}
		label += fmt.Sprintf("\\n%s\\n%s", ch.File, ch.TS)
		sb.WriteString(fmt.Sprintf("  \"%s\";\n", label))
	}

	sb.WriteString("  // Edges\n")
	for _, e := range g.Edges {
		sb.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [label=\"%s\"];\n", e.From, e.To, e.Label))
	}

	sb.WriteString("}\n")
	return sb.String()
}

func (g *GorutineGraph) getCloseBy(channelName string) (string, bool) {

	closeBy := make(map[string]string)
	for _, edge := range g.Edges {
		if edge.Label == "close" {
			closeBy[edge.To] = edge.From
		}
	}
	closer, ok := closeBy[channelName]
	return closer, ok
}
