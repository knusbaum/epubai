package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jaytaylor/html2text"
	"github.com/knusbaum/goreader/v2/epub"
	"github.com/knusbaum/goreader/v2/render"
)

func renderer(book epub.Rootfile) {
	renderer := render.New(&book.Package)
	renderer.RenderChapter(context.Background(), 3, os.Stdout)
}

var chapter = flag.Int("chapter", 0, "Start reading at this chapter")
var part = flag.Int("part", 0, "Start reading at this part of a given chapter")

func main() {
	flag.Parse()

	rc, err := epub.OpenReader(flag.Arg(0))
	if err != nil {
		fmt.Printf("Failed to open EPUB: %v\n", err)
		return
	}
	defer rc.Close()

	// The rootfile (content.opf) lists all of the contents of an epub file.
	// There may be multiple rootfiles, although typically there is only one.
	book := rc.Rootfiles[0]

	// Print book title.
	fmt.Println(book.Title)

	ctx, cancel := context.WithCancel(context.Background())
	handle_interrupt(ctx, cancel)

	aikey := os.Getenv("OPENAI_API_KEY")
	if aikey == "" {
		fmt.Printf("You must define the environment variable OPENAI_API_KEY\n")
		return
	}
	spk := NewSpeaker(aikey)

	var (
		lastchapt int
		lastpart  int
	)
	for k, item := range book.Spine.Itemrefs {
		if ctx.Err() != nil {

		}
		if k < *chapter {
			continue
		}
		rc, err := item.Open()
		if err != nil {
			log.Printf("Failed to open %v: %v\n", item.ID, err)
			continue
		}
		defer rc.Close()

		str, err := html2text.FromReader(rc)
		if err != nil {
			log.Printf("Failed to textualize: %v\n", err)
			continue
		}
		parts := strings.Split(str, "\n\n")
		for i, p := range parts {
			if ctx.Err() != nil {
				fmt.Printf("You may resume at Chapter %d, Part %d\n", lastchapt, lastpart)
				return
			}
			if k == *chapter && i < *part {
				continue
			}
			fmt.Printf("\n\nChapter %d | Part %d\n", k, i)

			lastchapt = k
			lastpart = i
			for _, part := range splitlen(p, 4096) {
				fmt.Print(part)
				if err := spk.Speak(ctx, part); err != nil {
					log.Printf("Failed to read book: %v\n", err)
					return
				}
			}
		}
	}
}

func splitlen(str string, max_chars int) []string {
	if len(str) < max_chars {
		return []string{str}
	}
	// Attempt to split on sentences
	parts := strings.Split(str, ". ")
	var out []string
	var part = parts[0] + ". "
	for _, p := range parts[1:] {
		if len(part+p) > max_chars {
			out = append(out, part)
			part = ""
		}
		part += p + ". "
	}
	out = append(out, part)
	return out
}

func handle_interrupt(ctx context.Context, onDisruption func()) {
	//fmt.Printf("Handling disruptions.\n")
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		defer signal.Stop(c)
		//defer fmt.Printf("Relinquishing control.\n")
		select {
		case <-c:
			fmt.Printf("\nInterrupted\n")
			onDisruption()
		case <-ctx.Done():
			return
		}
	}()
}
