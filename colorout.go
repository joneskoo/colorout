// Copyright 2016-2017 Joonas Kuorilehto.

// Command colorout is a colors and multiplexes output from tasks.
// Each task (external command) is executed concurrently using
// command shell and output is colored with a unique color per task.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/fatih/color"
)

var colors = []*color.Color{
	color.New(color.FgHiRed),
	color.New(color.FgHiGreen),
	color.New(color.FgHiYellow),
	color.New(color.FgHiBlue),
	color.New(color.FgHiMagenta),
	color.New(color.FgHiCyan),
	color.New(color.FgHiWhite),
	color.New(color.FgRed, color.ReverseVideo),
	color.New(color.FgGreen, color.ReverseVideo),
	color.New(color.FgYellow, color.ReverseVideo),
	color.New(color.FgBlue, color.ReverseVideo),
	color.New(color.FgMagenta, color.ReverseVideo),
	color.New(color.FgCyan, color.ReverseVideo),
	color.New(color.FgWhite, color.ReverseVideo),
}

func main() {
	fail := flag.Bool("fail", false, "terminate if any task fails with error")
	flag.Parse()
	tasks := flag.Args()

	if len(tasks) > len(colors) {
		log.Fatal("Too many tasks!")
	}

	// safeWriter protects stdout and stderr for concurrent access
	stdout := &safeWriter{W: os.Stdout}
	stderr := &safeWriter{W: os.Stderr}

	wg := &sync.WaitGroup{}
	wg.Add(len(tasks))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i, command := range tasks {
		fmt.Fprintf(stderr, "%d> Running: %s\n", i, command)

		go func(i int, command string) {
			defer wg.Done()
			if err := runCommand(ctx, i, command, stdout, stderr); err != nil {
				fmt.Fprintf(stderr, "%d> command failed with %v\n", i, err)
				if *fail { // terminate other tasks on failure
					cancel()
				}
			}
		}(i, command)
	}
	wg.Wait()
}

func runCommand(ctx context.Context, i int, command string, stdout io.Writer, stderr io.Writer) error {
	commandLine := append(shellCommand(), command)
	cmd := exec.CommandContext(ctx, commandLine[0], commandLine[1:]...)

	cmd.Stdout = colorize(stdout, i)
	cmd.Stderr = colorize(stderr, i)
	defer cmd.Stdout.(io.Closer).Close()
	defer cmd.Stderr.(io.Closer).Close()

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func shellCommand() []string {
	return []string{"bash", "-c"}
}

func colorize(dst io.Writer, i int) io.Writer {
	return &colorizer{
		dst: dst,
		i:   i,
	}
}

type colorizer struct {
	dst  io.Writer
	i    int
	prev string
}

func (c *colorizer) Write(data []byte) (n int, err error) {
	lines := strings.Split(c.prev+string(data), "\n")
	n = 0
	for _, line := range lines[:len(lines)-1] {
		n += len(line) + 1
		_, err = colors[c.i].Fprintf(c.dst, "%d> %v\n", c.i, line)
		if err != nil {
			return n, err
		}
	}
	c.prev = ""
	if n < len(data) {
		c.prev = string(data[n:])
	}
	n = len(data)
	return
}

func (c *colorizer) Close() error {
	if c.prev != "" {
		_, err := colors[c.i].Fprintf(c.dst, "%d> %v\n", c.i, c.prev)
		return err
	}
	return nil
}

type safeWriter struct {
	W io.Writer

	mu sync.Mutex
}

func (s *safeWriter) Write(data []byte) (n int, err error) {
	s.mu.Lock()
	n, err = s.W.Write(data)
	s.mu.Unlock()
	return
}
