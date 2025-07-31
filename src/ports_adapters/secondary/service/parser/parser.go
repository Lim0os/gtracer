package parser

import (
	"bufio"
	"fmt"
	"gtrace/src/domain/parser"
	"os"

	"io"
	"log/slog"
	"strings"
)

type Parser struct {
	input  io.Reader
	logger *slog.Logger
}

func NewParser(logger *slog.Logger) *Parser {
	return &Parser{logger: logger}
}

func (p *Parser) ParseFromCmd(input io.Reader) (*parser.GorutineGraph, error) {
	scanner := bufio.NewScanner(input)
	graph, err := p.ParseGorutineTrace(scanner)
	if err != nil {
		p.logger.Error("failed to parse graph", slog.String("error", err.Error()))
		return nil, err
	}
	return graph, nil
}

func (p *Parser) ParseFromFile(filePath string) (*parser.GorutineGraph, error) {
	file, err := os.Open(filePath)
	if err != nil {
		p.logger.Error("failed to open file", slog.String("error", err.Error()))
	}
	scanner := bufio.NewScanner(file)
	defer file.Close()

	graph, err := p.ParseGorutineTrace(scanner)
	if err != nil {
		p.logger.Error("failed to parse graph", slog.String("error", err.Error()))
		return nil, err
	}
	return graph, nil
}

func (p *Parser) ParseGorutineTrace(scanner *bufio.Scanner) (*parser.GorutineGraph, error) {
	graph := &parser.GorutineGraph{
		Gorutines: make(map[string]parser.Goroutine),
		Channels:  make(map[string]parser.Channel),
		Edges:     []parser.Edge{},
	}

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "[GTRACE]") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		switch parts[1] {
		case "channel_create":
			if len(parts) < 5 {
				return nil, fmt.Errorf("invalid channel_create format: %s", line)
			}
			channelName := fmt.Sprintf("chan_%s_%s", parts[2], parts[3])
			graph.Channels[channelName] = parser.Channel{
				Name: channelName,
				File: parts[3],
				TS:   parts[4],
			}

		case "func_start":
			if len(parts) < 6 {
				return nil, fmt.Errorf("invalid func_start format: %s", line)
			}
			goroutineID := parts[2]
			graph.Gorutines[goroutineID] = parser.Goroutine{
				ID:   goroutineID,
				Func: parts[3],
				File: parts[4],
				TS:   parts[5],
			}

		case "channel_send":
			if len(parts) < 5 {
				return nil, fmt.Errorf("invalid channel_send format: %s", line)
			}
			fromGoroutine := parts[2]
			channelName := fmt.Sprintf("chan_%s_%s", parts[3], parts[4])
			graph.Edges = append(graph.Edges, parser.Edge{
				From:  fromGoroutine,
				To:    channelName,
				Label: "send",
			})

		case "channel_close":
			if len(parts) < 5 {
				return nil, fmt.Errorf("invalid channel_close format: %s", line)
			}
			channelName := fmt.Sprintf("chan_%s_%s", parts[3], parts[4])
			if ch, ok := graph.Channels[channelName]; ok {
				ch.TS = parts[5]
				graph.Channels[channelName] = ch
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %v", err)
	}

	return graph, nil
}
