package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"

	"github.com/aerissecure/find"
)

// you can use dd to confirm match locations in a file:
// dd if=yourfile ibs=1 skip=<byte from match> count=100

var mat = regexp.MustCompile("matchme")
var sta = regexp.MustCompile("starting point")
var rel = regexp.MustCompile("relative match")

func main() {
	file, err := os.Open("lorem.txt")
	if err != nil {
		panic("error opening file")
	}

	g := find.NewGroup(
		find.NewMatcher(mat, 4, 83),
		find.NewMatcher(sta, 1, 0),
		find.NewMatcher(rel, 1, 0),
	)
	matches, err := g.Find(file)
	if err != nil {
		panic("error executing Find")
	}

	file.Seek(0, 0)
	txt, _ := ioutil.ReadAll(file)

	for _, m := range matches[0] {
		e := m.Byte + 7
		if int64(len(txt)) < m.Byte+7 {
			e = int64(len(txt))
		}
		fmt.Println(string(txt[m.Byte:e]))
	}

	for _, m := range matches[1] {
		e := m.Byte + 14
		if int64(len(txt)) < m.Byte+14 {
			e = int64(len(txt))
		}
		fmt.Println(string(txt[m.Byte:e]))
	}

	for _, m := range matches[2] {
		e := m.Byte + 14
		if int64(len(txt)) < m.Byte+14 {
			e = int64(len(txt))
		}
		fmt.Println(string(txt[m.Byte:e]))
	}
}
