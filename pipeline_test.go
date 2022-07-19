package pipeline_test

import (
	"bytes"
	"io"
	"pipeline"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// pipeline.ToJSON() is not testable cause the memo takes the current timestamp, which can't be faked

func TestTagIt(t *testing.T) {
	t.Parallel()
	m := pipeline.Memo{Modified: "09.07.2022 22:26:15", Content: "Hello world\n"}
	pure, err := pipeline.Marshal(m)
	if err != nil {
		t.Fatalf("want no error, got %q\n", err)
	}
	tags := []string{"test"}
	m.Tags = tags
	tagged, err := pipeline.Marshal(m)
	if err != nil {
		t.Fatalf("want no error, got %q\n", err)
	}
	w := pipeline.TagIt(tagged, "")
	if w.Error.Err != nil {
		t.Fatalf("want no error, got %q\n", w.Error.Err)
	}
	want := &bytes.Buffer{}
	w.Output = want
	w.Stdout()
	g := pipeline.TagIt(pure, "test")
	if g.Error.Err != nil {
		t.Fatalf("want no error from TagIt, got %q\n", g.Error.Err)
	}
	got := &bytes.Buffer{}
	g.Output = got
	g.Stdout()
	if !cmp.Equal(want.String(), got.String()) {
		t.Error(cmp.Diff(want.String(), got.String()))
	}
}

func TestGetAll(t *testing.T) {
	t.Parallel()
	w := make(map[string]pipeline.IndexEntry)
	w["30b2efc5b47dca50dd651d291e52b237f8fe59b98f8996ac234a6ca2a0d4af80"] = pipeline.IndexEntry{
		Tags: map[string]bool{
			"test": true,
		},
		Path:     "testdata/2022/07",
		Modified: "09.07.2022 22:26:15",
	}
	wa, err := pipeline.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	wan := &bytes.Buffer{}
	io.Copy(wan, wa)
	if err != nil {
		t.Fatal(err)
	}
	want := wan.String()
	want += "\n"
	////////////////////
	// want finished
	////////////////////

	g := pipeline.From(pipeline.PeriodAll, "testdata/index.dat")
	if g.Error.Err != nil {
		t.Fatalf("want no error from Get, but got %q", g.Error.Err)
	}
	gotten := &bytes.Buffer{}
	g.Output = gotten
	g.Stdout()
	got := gotten.String()
	////////////////////
	// got finished
	////////////////////

	if !cmp.Equal(want, got) {
		t.Error(cmp.Diff(want, got))
	}
}

func TestGetInvalidPath(t *testing.T) {
	t.Parallel()
	p := pipeline.From(pipeline.PeriodAll, "testdata/not_an_index.dat")
	if p.Error.Err == nil {
		t.Fatal("want error for non-existing index file from Get, but got none")
	}
}

func TestGetInvalidFilter(t *testing.T) {
	t.Parallel()
	p := pipeline.From("blub", "testdata/index.dat")
	if p.Error.Err == nil {
		t.Fatal("want error for invalid filter, but got none")
	}
}

func TestStdoutShort(t *testing.T) {
	t.Parallel()
	want := "\nHello world\n\n"
	i := pipeline.From(pipeline.PeriodAll, "testdata/index.dat")
	if i.Error.Err != nil {
		t.Fatalf("want no error for Get, but got %q\n", i.Error.Err)
	}

	g := pipeline.Stdout(i.Reader, "short")
	buf := &bytes.Buffer{}
	g.Output = buf
	g.Stdout()
	got := buf.String()

	if !cmp.Equal(want, got) {
		t.Errorf(cmp.Diff(want, got))
	}
}

// Found no negative scenario to test the errors from pipeline.TagIt()

/*
func TestStdout(t *testing.T) {
	t.Parallel()
	want := "Hello world!\n"
	p := pipeline.FromString(want)
	buf := &bytes.Buffer{}
	p.Output = buf
	p.Stdout()
	if p.Error != nil {
		t.Fatal(p.Error)
	}
	got := buf.String()
	if !cmp.Equal(want, got) {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestStdoutError(t *testing.T) {
	t.Parallel()
	p := pipeline.FromString("Hello world!\n")
	p.Error = errors.New("Oooh nooo!")
	buf := &bytes.Buffer{}
	p.Output = buf
	p.Stdout()
	got := buf.String()
	if got != "" {
		t.Errorf("want no output from stdout after error, but got %q", got)
	}
}

func TestFromFile(t *testing.T) {
	t.Parallel()
	want := []byte("Hello world!\n")
	p := pipeline.FromFile("testdata/hello.txt")
	if p.Error != nil {
		t.Fatal(p.Error)
	}
	got, err := io.ReadAll(p.Reader)
	if err != nil {
		t.Fatal(p.Error)
	}
	if !cmp.Equal(want, got) {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestFromFileInvalid(t *testing.T) {
	t.Parallel()
	p := pipeline.FromFile("testdata/nofile.txt")
	if p.Error == nil {
		t.Fatal("want error opening non-existing file, but got nil")
	}
}
*/
