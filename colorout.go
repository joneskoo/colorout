// Copyright 2016-2017 Joonas Kuorilehto.

// Command colorout is a colors and multiplexes output from tasks.
// Each task (external command) is executed concurrently using
// command shell and output is colored with a unique color per task.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
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
		colorOut := colorize(stdout, i)
		colorErr := colorize(stderr, i)
		fmt.Fprintf(colorErr, "%d> Running: %s\n", i, command)

		go func(i int, command string) {
			if err := runCommand(ctx, command, colorOut, colorErr); err != nil {
				fmt.Fprintf(stderr, "%d> command failed with %v\n", i, err)
				if *fail { // terminate other tasks on failure
					cancel()
				}
			}
			colorOut.Close()
			colorErr.Close()
			wg.Done()
		}(i, command)
	}
	wg.Wait()
}

func runCommand(ctx context.Context, command string, stdout, stderr io.Writer) error {
	commandLine := append(shellCommand(), command)
	cmd := exec.CommandContext(ctx, commandLine[0], commandLine[1:]...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
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

func colorize(dst io.Writer, i int) io.WriteCloser {
	return &colorizer{
		W:      dst,
		Prefix: fmt.Sprintf("%d> ", i),
		Color:  colors[i],
	}
}

type colorizer struct {
	W      io.Writer
	Prefix string
	Color  *color.Color

	trailer []byte
}

func (c *colorizer) write(prev, line []byte) (err error) {
	_, err = c.Color.Fprintf(c.W, "%s%s%s\n", c.Prefix, prev, line)
	return
}

// Write writes the contents of p into W with color coding.
// Each Stream is output with an unique color.
// If p does not end with a newline, the trailing partial line
// is buffered and will be output on next write or on Close.
func (c *colorizer) Write(p []byte) (n int, err error) {
	n = len(p)
	for {
		pos := bytes.IndexByte(p, '\n')
		if pos == -1 { // incomplete last line
			c.trailer = append(c.trailer[:0], p...)
			return
		}
		line := p[:pos]
		if err := c.write(c.trailer, line); err != nil {
			return n, err
		}
		p = p[pos+1:]
		c.trailer = nil
	}
}

// Close writes trailing data not terminated with a newline.
func (c *colorizer) Close() error {
	if len(c.trailer) > 0 {
		return c.write(c.trailer, nil)
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
