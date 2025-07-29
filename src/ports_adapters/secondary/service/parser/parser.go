package parser

import (
	"bufio"
	"fmt"
	"gtracer/src/domain/parser"
	"io"
	"strings"
)

type Parser struct {
	input io.Reader
}

func NewParser(input io.Reader) *Parser {
	return &Parser{input: input}
}

func (p *Parser) Parse() {
	scanner := bufio.NewScanner(p.input)
	p.ParseToDot(scanner)
}

//func (p *Parser) File() {
//	file, err := os.Open(p.filePath)
//	if err != nil {
//		panic(err)
//	}
//	defer file.Close()
//
//	scanner := bufio.NewScanner(file)
//	p.ParseToDot(scanner)
//}

func (p *Parser) ParseToDot(scanner *bufio.Scanner) {
	goroutines := make(map[string]parser.Goroutine)
	channels := make(map[string]parser.Channel)
	edges := []parser.Edge{}
	channelClosedBy := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "[GTRACE]") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		event := parts[1]

		switch event {
		case "channel_create":

			ch := parser.Channel{
				Name: parts[2],
				File: parts[3],
				TS:   parts[4],
			}
			channels[ch.Name] = ch

		case "func_start":
			goroutines[parts[2]] = parser.Goroutine{
				ID:   parts[2],
				Func: parts[3],
				File: parts[4],
				TS:   parts[5],
			}

		case "func_end":
			edges = append(edges, parser.Edge{
				From:  fmt.Sprintf("goroutine %s (ID: %s)\n%s\n%s", goroutines[parts[2]].Func, parts[2], goroutines[parts[2]].File, goroutines[parts[2]].TS),
				To:    fmt.Sprintf("end %s (ID: %s)\n%s\n%s", goroutines[parts[2]].Func, parts[2], parts[4], parts[5]),
				Label: "end",
			})

		case "channel_send":
			edges = append(edges, parser.Edge{
				From:  fmt.Sprintf("goroutine %s (ID: %s)\n%s\n%s", goroutines[parts[2]].Func, parts[2], goroutines[parts[2]].File, goroutines[parts[2]].TS),
				To:    fmt.Sprintf("channel %s", parts[3]),
				Label: "send",
			})

		case "channel_receive":
			edges = append(edges, parser.Edge{
				From:  fmt.Sprintf("channel %s", parts[3]),
				To:    fmt.Sprintf("goroutine %s (ID: %s)\n%s\n%s", goroutines["1"].Func, "1", parts[4], parts[5]),
				Label: "receive",
			})

		case "channel_close":
			channelClosedBy[parts[3]] = goroutines[parts[2]].Func
			edges = append(edges, parser.Edge{
				From:  fmt.Sprintf("goroutine %s (ID: %s)\n%s\n%s", goroutines[parts[2]].Func, parts[2], goroutines[parts[2]].File, goroutines[parts[2]].TS),
				To:    fmt.Sprintf("channel %s", parts[3]),
				Label: "close",
			})
		}
	}

	fmt.Println("strict digraph goroutine_channels {")
	fmt.Println("  // Nodes")
	for _, g := range goroutines {
		fmt.Printf("  \"%s (ID: %s)\\n%s\\n%s\";\n", g.Func, g.ID, g.File, g.TS)
	}
	for _, ch := range channels {
		label := fmt.Sprintf("channel %s", ch.Name)
		if closer, ok := channelClosedBy[ch.Name]; ok {
			label += fmt.Sprintf(" (closed by %s)", closer)
		}
		label += fmt.Sprintf("\\n%s\\n%s", ch.File, ch.TS)
		fmt.Printf("  \"%s\";\n", label)
	}

	fmt.Println("  // Edges")
	for _, e := range edges {
		fmt.Printf("  \"%s\" -> \"%s\" [label=\"%s\"];\n", e.From, e.To, e.Label)
	}
	fmt.Println("}")
}
