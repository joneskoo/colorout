// Copyright 2016 Joonas Kuorilehto

// Colorout is a command line utility that colors command
// output. Each command is executed concurrently using bash
// shell and annotated with a different color.
package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

const shell = "bash"

const (
	colorReset   = "\x1b[0m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorWhite   = "\033[37m"
)

var colors = []string{
	colorRed,
	colorGreen,
	colorYellow,
	colorBlue,
	colorMagenta,
	colorCyan,
	colorWhite}

var mu sync.Mutex

func main() {
	log.SetFlags(0)
	var wg sync.WaitGroup
	args := os.Args[1:]
	if len(args) > len(colors) {
		log.Fatal("Too many commands!")
	}
	for i, command := range args {
		wg.Add(1)
		go func(i int, command string) {
			defer wg.Done()
			runCommand(i, command)
		}(i, command)
	}
	wg.Wait()
}

func runCommand(i int, command string) {
	out(os.Stdout, i, fmt.Sprint("Running: ", command))
	cmd := exec.Command(shell, "-c", command)

	// Close standard input
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	stdin.Close()

	// Read standard output and error
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}

	err = cmd.Start()
	if err != nil {
		log.Fatal(err)
	}

	go readOutput(os.Stdout, i, stdout)
	go readOutput(os.Stderr, i, stderr)

	err = cmd.Wait()
	if err != nil {
		log.Fatalf("%d Command finished with error %v", i, err)
	}
	out(os.Stdout, i, "Command exited successfully")
}

func readOutput(file *os.File, i int, r io.Reader) {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			break
		}
		out(file, i, strings.TrimRight(line, "\n"))
	}
}

func out(file *os.File, i int, line string) {
	if i > len(colors) {
		panic("Color index out of range")
	}

	mu.Lock()
	defer mu.Unlock()
	file.WriteString(fmt.Sprintf("%s%d> %s%s\n", colors[i], i, line, colorReset))
}
