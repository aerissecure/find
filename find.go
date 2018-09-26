// Package find provides concurrent streaming pattern matching on an io.Reader
// using standard library regexp patterns

package find

import (
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"sync"

	"github.com/aerissecure/unreader"
)

// Matches returns all successive matches of the expression as a slice
// of strings. If n >= 0, the function returns at most n matches, otherwise it
// returns all of them. If n != 0, the reader may not be completely read.
func Find(rdr io.Reader, re *regexp.Regexp, n int) []Match {
	// TODO: change unreader and circbuf to be uint to avoid error
	r, _ := unreader.NewUnreader(1024, rdr)

	matches := make([]Match, 0, n)
	for n == 0 || len(matches) < n {
		sbyte := r.Cursor()
		matchIdx := re.FindReaderIndex(r)
		if matchIdx == nil {
			break
		}
		ebyte := r.Cursor()
		bytesRead := ebyte - sbyte
		bytesNeed := matchIdx[1]
		extra := bytesRead - int64(bytesNeed)

		r.Unread(extra)

		l := matchIdx[1] - matchIdx[0]
		txt := string(r.LastBytes(l))

		matches = append(matches, Match{Text: txt, Byte: ebyte - extra - int64(l)})
	}
	return matches
}

type Match struct {
	Text string
	Byte int64 // this is the location of the starting byte, NOT the rune index
}

type Matcher struct {
	regexp *regexp.Regexp
	count  int
	size   int64
}

func NewMatcher(re *regexp.Regexp, count int, size int64) *Matcher {
	return &Matcher{
		regexp: re,
		count:  count,
		size:   size,
	}
}

// Group house the methods and state needed to manage multiple streaming regex
// searches against a single reader
type Group struct {
	matchers []*Matcher
	matches  map[int][]Match
	mu       sync.Mutex
}

// we need a way to register regexes. instead of wwrapping it and passing in a name or something,
// we will perform all operations using the index of the regex. So if you are passing into the
// variadic function, we assuming the caller knows the index of each. If you want the index
// explicitly returned, use the AddRegexp method.
func NewGroup(mm ...*Matcher) *Group {
	return &Group{
		matchers: mm,
		matches:  make(map[int][]Match),
	}
}

// AddRegexp adds a Matcher/expression to the find group and returns the index
// of the Matcher which can be used to lookup matching the output.
func (g *Group) AddMatcher(m *Matcher) int {
	g.matchers = append(g.matchers, m)
	return len(g.matchers) - 1
}

func (g *Group) appendMatch(idx int, m Match) {
	g.mu.Lock()
	g.matches[idx] = append(g.matches[idx], m)
	g.mu.Unlock()
}

// Find performs concurrent matching of the configured matchers against the
// io.Reader. A map zero indexed by the order that the matchers were configured
// is returned with a slice of Match objects as their keys.
func (g *Group) Find(r io.Reader) (map[int][]Match, error) {
	var wg sync.WaitGroup
	writers := make([]*io.PipeWriter, len(g.matchers))

	for i, m := range g.matchers {
		mr, mw := io.Pipe()
		writers[i] = mw
		wg.Add(1)

		go func(idx int, mr io.Reader, m *Matcher) {
			lr := mr
			if m.size > 0 {
				lr = io.LimitReader(mr, m.size)
			}
			for _, match := range Find(lr, m.regexp, m.count) {
				g.appendMatch(idx, match)
			}
			// EOF won't be reached if FindAllReaderString aborts early due to a limit
			// on number of matches, causing deadlock. So read to the end of the file.
			io.Copy(ioutil.Discard, mr) // drain underlying reader, not io.ReadLimiter
			wg.Done()
		}(i, mr, m)
	}

	// cannot cast []*io.PipeWriter to []io.Writer
	// https://stackoverflow.com/questions/42335756/passing-io-pipewriter-to-io-multiwriter
	ws := make([]io.Writer, len(g.matchers))
	for i, w := range writers {
		ws[i] = w
	}

	mw := io.MultiWriter(ws...)

	// io.Copy completes when r is exhausted, but EOF is not sent until the
	// io.PipeWriters are closed. This call won't deadlock on its own.
	if _, err := io.Copy(mw, r); err != nil {
		return g.matches, fmt.Errorf("io.Copy error: %s", err)
	}

	// send EOF to io.PipeReaders
	for _, w := range writers {
		w.Close()
	}

	// wait for go routines to return (e.g. add matches)
	wg.Wait()
	return g.matches, nil
}
