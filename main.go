package main

import (
	"fmt"
	"gtracer/src/ports_adapters/secondary/service/instrumented"
	"gtracer/src/ports_adapters/secondary/service/parser"
	"io"
	"log"
	"os"
	"os/exec"
)

func main() {

	inst := instrumented.New("../test_gorutine1", "", nil)
	inst.Processed()
	log.Println(fmt.Sprintf("%s/main.go", inst.OutputPath))

	cmd := exec.Command("sh", "-c", fmt.Sprintf("cd %s && go run main.go", inst.OutputPath))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	pr, pw := io.Pipe()
	multiOut := io.MultiWriter(pw, os.Stdout)

	go func() {
		if _, err := io.Copy(multiOut, stdout); err != nil {
			log.Printf("copy error: %v", err)
		}
		pw.Close()
	}()

	parse := parser.NewParser(pr)
	parse.Parse()

	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}
}
