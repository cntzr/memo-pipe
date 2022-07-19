package pipeline

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type (
	Pipeline struct {
		Reader io.Reader
		Output io.Writer
		Error  MaskedError
	}

	// Memo ... describes a default memo containing every kind of information
	Memo struct {
		Tags     []string
		Modified string // maybe created or updated
		Content  string
	}

	IndexEntry struct {
		Tags     map[string]bool
		Path     string
		Modified string
	}

	Index struct {
		entries map[string]IndexEntry
	}

	MaskedError struct {
		Prefix string
		Err    error
	}

	Period string
)

var (
	// Lock for index file
	lock sync.Mutex

	// Marshal is a function that marshals the object into an io.Reader.
	// By default, it uses the JSON marshaller.
	Marshal = func(v interface{}) (io.Reader, error) {
		b, err := json.MarshalIndent(v, "", "\t")
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(b), nil
	}

	// Unmarshal is a function that unmarshals the data from the reader into the specified value.
	// By default, it uses the JSON unmarshaller.
	Unmarshal = func(r io.Reader, v interface{}) error {
		return json.NewDecoder(r).Decode(v)
	}

	validPeriod = map[Period]bool{
		PeriodAll:           true,
		PeriodToday:         true,
		PeriodYesterday:     true,
		PeriodThisWeek:      true,
		PeriodLastWeek:      true,
		PeriodLastTwoWeeks:  true,
		PeriodThisMonth:     true,
		PeriodLastMonth:     true,
		PeriodLastTwoMonths: true,
		PeriodThisYear:      true,
		PeriodLastYear:      true,
		PeriodLastTwoYears:  true,
	}
)

const (
	basePath  = "~/.local/share/memo"
	indexFile = "index.dat"

	// Time periods for Get
	PeriodAll           Period = "all"
	PeriodToday         Period = "today"
	PeriodYesterday     Period = "yesterday"
	PeriodThisWeek      Period = "thisweek"
	PeriodLastWeek      Period = "lastweek"
	PeriodLastTwoWeeks  Period = "last2weeks"
	PeriodThisMonth     Period = "thismonth"
	PeriodLastMonth     Period = "lastmonth"
	PeriodLastTwoMonths Period = "last2months"
	PeriodThisYear      Period = "thisyear"
	PeriodLastYear      Period = "lastyear"
	PeriodLastTwoYears  Period = "last2years"
)

/*
  "diagnostic.errorSign": "✘",
*/

//////////////////////////////////////////////////////////
// ToJSON ... it's more a "from stdin to stdout as json"
// used by memo
//////////////////////////////////////////////////////////
func ToJSON(rd io.Reader) *Pipeline {
	content := &bytes.Buffer{}
	reader := bufio.NewReader(rd)
	scanner := bufio.NewScanner(reader)
	// Scan all input into content
	for scanner.Scan() {
		line := scanner.Text()
		// >>> place to do anything with the line >>>
		/// line = strings.ToUpper(line)
		// <<< place to do anything with the line <<<
		fmt.Fprintln(content, line)
	}
	// Build a memo
	memo := Memo{
		Modified: time.Now().Format("02.01.2006 15:04:05"),
		Content:  content.String(),
	}
	// Marshal the structured memo to JSON
	r, err := Marshal(memo)
	if err != nil {
		return &Pipeline{
			Error: MaskedError{
				Prefix: "✘ error ... ",
				Err:    err,
			},
		}
	}
	return &Pipeline{
		Reader: r,
	}
}

//////////////////////////////////////////////////////////
// TagIt ... add a tag to an existing memo
// used by tagit
//////////////////////////////////////////////////////////
func TagIt(rd io.Reader, tag string) *Pipeline {
	memo := Memo{}
	err := Unmarshal(rd, &memo)
	if err != nil {
		return &Pipeline{
			Error: MaskedError{
				Prefix: "✘ error ... ",
				Err:    err,
			},
		}
	}
	// Tag the memo
	if tag != "" {
		memo.Tags = append(memo.Tags, tag)
	}
	// Marshal the structured memo back to JSON
	r, err := Marshal(memo)
	if err != nil {
		return &Pipeline{
			Error: MaskedError{
				Prefix: "✘ error ... ",
				Err:    err,
			},
		}
	}
	return &Pipeline{
		Reader: r,
	}
}

//////////////////////////////////////////////////////////
// Keep ... index & store a memo
// used by keep
//////////////////////////////////////////////////////////
func Keep(rd io.Reader) {
	memo := Memo{}
	err := Unmarshal(rd, &memo)
	// Configure & create the path for the memo
	filePath, err := memo.GetPath(basePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✘ error ... %q\n", err)
		os.Exit(1)
	}
	os.MkdirAll(filePath, os.ModePerm)
	// Hash the content as part of the filename
	hash, err := memo.Hash()
	if err != nil {
		fmt.Fprintf(os.Stderr, "✘ error ... %q\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ hashed ... %s\n", hash)

	// Fill the index
	idxFile := TakeMeHome(basePath + string(os.PathSeparator) + indexFile)
	// Loading the index from file
	idx := NewIndex()
	err = idx.Load(idxFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✘ error ... %q\n", err)
		os.Exit(1)
	}
	defer idx.Store(idxFile)
	idxEntry := IndexEntry{
		Tags:     make(map[string]bool),
		Path:     filePath,
		Modified: memo.Modified,
	}
	for _, tag := range memo.Tags {
		idxEntry.Tags[tag] = true
	}
	before := len(idx.entries)
	idx.Upsert(hash, idxEntry)
	after := len(idx.entries)
	if before < after {
		fmt.Fprintf(os.Stderr, "✓ index ... added, %d entries now\n", after)
	} else {
		fmt.Fprintf(os.Stderr, "✓ index ... updated, %d entries\n", after)
	}
	// Find a file for the memo
	fi, err := os.Create(filePath + string(os.PathSeparator) + hash + ".memo")
	if err != nil {
		fmt.Fprintf(os.Stderr, "✘ error ... %q\n", err)
		os.Exit(1)
	}
	defer fi.Close()
	// Finally write the Memo
	err = memo.Write(fi)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✘ error ... %q\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "✓ stored")
}

////////////////////////////////////////////////////////////////
// From ... take all or a period part of the index into memory
// if filePath is empty, it will be set with a perfect default
// used by from
////////////////////////////////////////////////////////////////
func From(period Period, filePath string) *Pipeline {
	// Help wanted?
	if period == "help" {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "FROM is part of the memo's famous toolbox")
		fmt.Fprintln(os.Stderr, "-----------------------------------------")
		fmt.Fprintln(os.Stderr, "Usage: from <period> | further tools")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "It reads an index file for memos. Entries get filtered by periods of time. The")
		fmt.Fprintln(os.Stderr, "periods are described verbally. Dates or timestamps are not supported.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Following periods are supported by now ...")
		fmt.Fprintln(os.Stderr, "  ✓ all (default)")
		fmt.Fprintln(os.Stderr, "  ✘ today")
		fmt.Fprintln(os.Stderr, "  ✘ yesterday")
		fmt.Fprintln(os.Stderr, "  ✘ thisweek")
		fmt.Fprintln(os.Stderr, "  ✘ lastweek")
		fmt.Fprintln(os.Stderr, "  ✘ last2weeks")
		fmt.Fprintln(os.Stderr, "  ✘ thismonth")
		fmt.Fprintln(os.Stderr, "  ✘ lastmonth")
		fmt.Fprintln(os.Stderr, "  ✘ last2months")
		fmt.Fprintln(os.Stderr, "  ✘ thisyear")
		fmt.Fprintln(os.Stderr, "  ✘ lastyear")
		fmt.Fprintln(os.Stderr, "  ✘ last2years")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Memo's toolbox contains these tool chains ...")
		fmt.Fprintln(os.Stderr, "  ... memo, taggit, keep")
		fmt.Fprintln(os.Stderr, "  ... from, tagged, stdout")
		fmt.Fprintln(os.Stderr, "It's inspired by Napoleon as a partner of Timothy Truckle and the more modern")
		fmt.Fprintln(os.Stderr, "aunties in Gibson's trilogy about stubs.")
		fmt.Fprintln(os.Stderr)
		os.Exit(0)
	}
	// Check the period
	if !validPeriod[period] {
		return &Pipeline{
			Error: MaskedError{
				Prefix: "✘ error ... ",
				Err:    fmt.Errorf("unknown period %v", period),
			},
		}
	}

	// Open the index file
	fi := &os.File{}
	if filePath == "" {
		filePath = TakeMeHome(basePath + string(os.PathSeparator) + indexFile)
	}
	fi, err := os.Open(filePath)
	if err != nil {
		return &Pipeline{
			Error: MaskedError{
				Prefix: "✘ error ... ",
				Err:    err,
			},
		}
	}
	defer fi.Close()

	// Pull all index entries into a map
	entries := make(map[string]IndexEntry)
	err = Unmarshal(fi, &entries)
	if err != nil {
		return &Pipeline{
			Error: MaskedError{
				Prefix: "✘ error ... ",
				Err:    err,
			},
		}
	}

	// Run the period filter over the map
	switch period {
	case PeriodAll:
		r, err := Marshal(entries)
		if err != nil {
			return &Pipeline{
				Error: MaskedError{
					Prefix: "✘ error ... ",
					Err:    err,
				},
			}
		}
		/////////////////////////////////////////////////////////////////
		// That's the only positive return of this method in the moment
		/////////////////////////////////////////////////////////////////
		return &Pipeline{
			Reader: r,
		}
	}

	// This should not happen due to a lot of cool decisions before
	return &Pipeline{
		Error: MaskedError{
			Prefix: "✘ error ... ",
			Err:    fmt.Errorf("unexpected period %q, we should really work on it", period),
		},
	}
}

//////////////////////////////////////////////////////////
// Tagged ... reduces an index to tagged entries only
// used by tagged
//////////////////////////////////////////////////////////
func Tagged(rd io.Reader, tags ...string) *Pipeline {
	entries := make(map[string]IndexEntry)
	err := Unmarshal(rd, &entries)
	if err != nil {
		return &Pipeline{
			Error: MaskedError{
				Prefix: "✘ error ... ",
				Err:    err,
			},
		}
	}

	found := make(map[string]IndexEntry)
	for key, value := range entries {
		hit := true
		for _, tag := range tags {
			if !value.Tags[tag] {
				hit = false
				break
			}
		}
		if hit {
			found[key] = value
		}
	}

	filtered, err := Marshal(found)
	if err != nil {
		return &Pipeline{
			Error: MaskedError{
				Prefix: "✘ error ... ",
				Err:    err,
			},
		}
	}

	return &Pipeline{
		Reader: filtered,
	}
}

//////////////////////////////////////////////////////////
// Stdout ... prints one ore more memos in the terminal
// selection will be triggered by incoming index entries
// used by stdout
//////////////////////////////////////////////////////////
func Stdout(rd io.Reader, what string) *Pipeline {
	//////////////////////////////////////////////////////
	// Handle input vi pipe from previous process
	// Should be a list of index entries
	//////////////////////////////////////////////////////
	idx := map[string]IndexEntry{}
	err := Unmarshal(rd, &idx)
	if err != nil {
		return &Pipeline{
			Error: MaskedError{
				Prefix: "✘ error ... ",
				Err:    err,
			},
		}
	}
	//////////////////////////////////////////////////////
	// Load memos from files
	// coordinates come from index entries
	//////////////////////////////////////////////////////
	memos := []Memo{}
	for fileName, details := range idx {
		fi, err := os.Open(details.Path + string(os.PathSeparator) + fileName + ".memo")
		if err != nil {
			return &Pipeline{
				Error: MaskedError{
					Prefix: "✘ error ... ",
					Err:    err,
				},
			}
		}
		defer fi.Close()
		memo := Memo{}
		err = Unmarshal(fi, &memo)
		if err != nil {
			return &Pipeline{
				Error: MaskedError{
					Prefix: "✘ error ... ",
					Err:    err,
				},
			}
		}
		memos = append(memos, memo)
	}
	buf := &bytes.Buffer{}
	for _, memo := range memos {

		//////////////////////////////////////////////////////////////////////
		// Switch verbosity of the output
		//   short   ... memo only (default)
		//   long    ... all fields
		//   verbose ... all fields with labels
		//////////////////////////////////////////////////////////////////////
		switch what {
		case "long":
			_, err := fmt.Fprintf(buf, "\n%s\n%s", memo.Modified, memo.Content)
			if err != nil {
				return &Pipeline{
					Error: MaskedError{
						Prefix: "✘ error ... ",
						Err:    err,
					},
				}
			}
		case "verbose":
			_, err := fmt.Fprintf(buf, "\nModified: %s\nTags: %s\nMemo: %s",
				memo.Modified,
				strings.Join(memo.Tags, ", "),
				memo.Content)
			if err != nil {
				return &Pipeline{
					Error: MaskedError{
						Prefix: "✘ error ... ",
						Err:    err,
					},
				}
			}
		default:
			_, err := fmt.Fprintf(buf, "\n%s", memo.Content)
			if err != nil {
				return &Pipeline{
					Error: MaskedError{
						Prefix: "✘ error ... ",
						Err:    err,
					},
				}
			}
		}
	}
	return &Pipeline{
		Reader: buf,
	}
}

//////////////////////////////////////////////////////////
// TakeMeHome ... replaces the leading ~ in a path with
// user's home dir
//////////////////////////////////////////////////////////
func TakeMeHome(path string) string {
	if string(path[0]) != "~" {
		return path
	}
	home := os.Getenv("HOME")
	if home == "" {
		return path
	}
	path = home + path[1:]
	return path
}

//////////////////////////////////////////////////////
// PIPELINE
//////////////////////////////////////////////////////

// Stdout ... Universal output, per default to Stdout
// but it could be any other io.Writer, too
func (p *Pipeline) Stdout() {
	if p.Error.Err != nil {
		return
	}
	io.Copy(p.Output, p.Reader)
	// Cosmetic improvements at the output
	lineBreak := bytes.Buffer{}
	lineBreak.WriteString("\n")
	io.Copy(p.Output, &lineBreak)
}

//////////////////////////////////////////////////////
// INDEX
//////////////////////////////////////////////////////

// NewIndex ... Constructor for Index
func NewIndex() *Index {
	entries := make(map[string]IndexEntry)
	return &Index{entries: entries}
}

// Upsert ... Inserts a new entry or overwrites an existing entry in the index
func (i *Index) Upsert(key string, e IndexEntry) {
	i.entries[key] = e
}

// Delete ... Removes an entry from the index
func (i *Index) Delete(key string) error {
	_, exist := i.entries[key]
	if !exist {
		return fmt.Errorf("index entry for key %s does not exist", key)
	}
	delete(*&i.entries, key)
	return nil
}

// Find ... Looks for an entry in the index by key
func (i *Index) Find(key string) IndexEntry {
	return i.entries[key]
}

// Store ... Stores entries into file at path
func (i *Index) Store(path string) error {
	lock.Lock()
	defer lock.Unlock()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	r, err := Marshal(i.entries)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	return err
}

// Load ... Fills the entries from file at path
func (i *Index) Load(path string) error {
	lock.Lock()
	defer lock.Unlock()
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()
	err = Unmarshal(f, &i.entries)
	if err != nil {
		return err
	}
	return nil
}

//////////////////////////////////////////////////////
// MEMO
//////////////////////////////////////////////////////

// Write ... stores a memo into a file
func (m *Memo) Write(w io.Writer) error {
	enc := json.NewEncoder(w)
	return enc.Encode(m)
}

// Read ... reads a memo from a file
func (m *Memo) Read(r io.Reader) error {
	dec := json.NewDecoder(r)
	return dec.Decode(m)
}

// Hash ... calculates the hash sum of a memo's content
func (m *Memo) Hash() (hash string, err error) {
	buf := bytes.Buffer{}
	enc := json.NewEncoder(&buf)
	err = enc.Encode(m.Content)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write(buf.Bytes())
	hash = fmt.Sprintf("%x", h.Sum(nil))
	return hash, nil
}

// GetPath ... creates a storage path for the memo
func (m *Memo) GetPath(path string) (string, error) {
	path = TakeMeHome(path)
	created, err := time.Parse("02.01.2006 15:04:05", m.Modified)
	if err != nil {
		return "", err
	}
	year := created.Format("2006")
	month := created.Format("01")
	path = strings.Join([]string{path, year, month}, string(os.PathSeparator))
	return path, nil
}

//////////////////////////////////////////////////////
// MULTIERROR
//////////////////////////////////////////////////////

func (e *MaskedError) Error() string {
	return e.Prefix + e.Error()
}
