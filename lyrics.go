// lyrics scrapes genius.com to find a song and print its lyrics.
//
// Written by Artem Krinitsyn. Public domain.
package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
)

type Song struct {
	URL string `json:"url"`
	// Title string `json:"title"`
}

type SearchResponse struct {
	Response struct {
		Sections []struct {
			Type string `json:"type"`
			Hits []struct {
				Result Song `json:"result"`
			} `json:"hits"`
		} `json:"sections"`
	} `json:"response"`
}

func searchSong(name string) (*Song, error) {
	url := "https://genius.com/api/search/song?page=1&q=" + url.QueryEscape(name)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var sr SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}

	if len(sr.Response.Sections) == 0 {
		panic("Response.Sections is empty")
	}
	section := sr.Response.Sections[0]
	if len(section.Hits) == 0 {
		return nil, errors.New("no song found")
	}
	return &section.Hits[0].Result, nil
}

func extractMain(doc []byte) ([]byte, error) {
	reStart := regexp.MustCompile(`<main[^>]*>`)
	reEnd := regexp.MustCompile(`</main>`)

	startMatch := reStart.FindIndex(doc)
	if startMatch == nil {
		return nil, errors.New("no <main> start tag found")
	}

	endMatch := reEnd.FindIndex(doc)
	if endMatch == nil {
		return nil, errors.New("no </main> end tag found")
	}

	startPos := startMatch[1]
	endPos := endMatch[0]

	if startPos > endPos {
		return nil, errors.New("end tag comes before start tag")
	}

	return doc[startPos:endPos], nil
}

// TODO: accept io.Reader.
func printLyrics(doc []byte) error {
	mainContent, err := extractMain(doc)
	if err != nil {
		return err
	}

	d := xml.NewDecoder(bytes.NewReader(mainContent))
	d.Strict = false
	d.AutoClose = xml.HTMLAutoClose
	d.Entity = xml.HTMLEntity

	captureContent := false
	depth := 0
	capturedSomething := false
	for {
		token, err := d.Token()
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "div" {
				if captureContent {
					depth++
					break
				}
				for _, attr := range t.Attr {
					if attr.Name.Local == "data-lyrics-container" {
						captureContent = true
						depth = 0
						break
					}
				}
			}
			if captureContent && t.Name.Local == "br" {
				fmt.Println()
			}

		case xml.EndElement:
			if t.Name.Local == "div" {
				if captureContent && depth > 0 {
					depth--
				} else if captureContent && depth == 0 {
					if capturedSomething {
						// add newline only if the block was not empty
						fmt.Println()
						capturedSomething = false
					}
					captureContent = false
				}
			}

		case xml.CharData:
			if captureContent && depth == 0 {
				capturedSomething = true
				fmt.Print(string(t))
			}
		}
	}
	return nil
}

func main() {
	log.SetPrefix(os.Args[0] + ": ")
	log.SetFlags(0)

	if len(os.Args) < 2 {
		log.Fatal("no song name is provided")
	}
	// TODO: support providing multiple songs.
	name := os.Args[1]
	song, err := searchSong(name)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := http.Get(song.URL)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	printLyrics(b)
}
