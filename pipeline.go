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
		Error  error
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
)

const (
	basePath  = "~/.local/share/memo"
	indexFile = "index.dat"
)

// FromStdinToJSON ... it's more a "from stdin to stdout as json"
// used by memo
func FromStdinToJSON() *Pipeline {
	content := &bytes.Buffer{}
	reader := bufio.NewReader(os.Stdin)
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
	return &Pipeline{
		Reader: r,
		Error:  err,
	}
}

// FromStdinTagIt ... add a tag to an existing memo
// used by tagit
func FromStdinTagIt(tag string) *Pipeline {
	memo := Memo{}
	reader := bufio.NewReader(os.Stdin)
	prevErr := json.NewDecoder(reader).Decode(&memo)
	// Tag th memo
	memo.Tags = append(memo.Tags, tag)
	// Marshal the structured memo back to JSON
	r, err := Marshal(memo)
	if err != nil && prevErr != nil {
		err = fmt.Errorf("× errors ...\n  ... %w\n  ... %w\n", prevErr, err)
	}
	if err == nil && prevErr != nil {
		err = fmt.Errorf("× error ... %w\n", prevErr)
	}
	return &Pipeline{
		Reader: r,
		Error:  err,
	}
}

// FromStdinKeep ... index & store a memo
// used by keep
func FromStdinKeep() {
	memo := Memo{}
	reader := bufio.NewReader(os.Stdin)
	err := json.NewDecoder(reader).Decode(&memo)
	// Configure & create the path for the memo
	filePath, err := memo.GetPath(basePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "- error ... %q\n", err)
		os.Exit(1)
	}
	os.MkdirAll(filePath, os.ModePerm)
	// Hash the content as part of the filename
	hash, err := memo.Hash()
	if err != nil {
		fmt.Fprintf(os.Stderr, "- error ... %q\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ hashed ... %s\n", hash)

	// Fill the index
	idxFile := TakeMeHome(basePath + string(os.PathSeparator) + indexFile)
	// Loading the index from file
	idx := NewIndex()
	err = idx.Load(idxFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "× error ... %q\n", err)
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
		fmt.Fprintf(os.Stderr, "× error ... %q\n", err)
		os.Exit(1)
	}
	defer fi.Close()
	// Finally write the Memo
	err = memo.Write(fi)
	if err != nil {
		fmt.Fprintf(os.Stderr, "× error ... %q\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "✓ stored")
}

// TakeMeHome ... replaces the leading ~ in a path with user's home dir
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
	if p.Error != nil {
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
