package parser

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
