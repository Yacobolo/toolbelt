package sourcebook

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestListWithFormat(t *testing.T) {
	t.Parallel()

	app := New(t.TempDir(), &fakeCloner{contents: map[string]string{
		"https://example.com/acme/alpha.git": "alpha source",
	}})
	if err := app.Add(context.Background(), "https://example.com/acme/alpha.git"); err != nil {
		t.Fatal(err)
	}

	var table bytes.Buffer
	if err := app.ListWithFormat(&table, ListFormatTable); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(table.String(), "NAME") ||
		!strings.Contains(table.String(), "PROVIDER") ||
		!strings.Contains(table.String(), "UPDATED") ||
		!strings.Contains(table.String(), "alpha  git") ||
		!strings.Contains(table.String(), "12 B") {
		t.Fatalf("table output = %q", table.String())
	}

	var tsv bytes.Buffer
	if err := app.ListWithFormat(&tsv, ListFormatTSV); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tsv.String(), "alpha\tgit\thttps://example.com/acme/alpha.git\t") ||
		!strings.HasSuffix(tsv.String(), "\t12\n") {
		t.Fatalf("TSV output = %q", tsv.String())
	}

	var jsonOutput bytes.Buffer
	if err := app.ListWithFormat(&jsonOutput, ListFormatJSON); err != nil {
		t.Fatal(err)
	}
	var records []struct {
		Name      string `json:"name"`
		Provider  string `json:"provider"`
		URL       string `json:"url"`
		SizeBytes int64  `json:"size_bytes"`
	}
	if err := json.Unmarshal(jsonOutput.Bytes(), &records); err != nil {
		t.Fatalf("JSON output = %q: %v", jsonOutput.String(), err)
	}
	if len(records) != 1 || records[0].Name != "alpha" || records[0].Provider != ProviderGit ||
		records[0].URL != "https://example.com/acme/alpha.git" || records[0].SizeBytes != 12 {
		t.Fatalf("JSON records = %#v", records)
	}
}

func TestListWithFormatEmptyOutput(t *testing.T) {
	t.Parallel()

	app := New(t.TempDir(), &fakeCloner{})
	var table bytes.Buffer
	if err := app.ListWithFormat(&table, ListFormatTable); err != nil {
		t.Fatal(err)
	}
	if got, want := table.String(), "No sources configured.\n"; got != want {
		t.Fatalf("empty table = %q, want %q", got, want)
	}

	var tsv bytes.Buffer
	if err := app.ListWithFormat(&tsv, ListFormatTSV); err != nil {
		t.Fatal(err)
	}
	if tsv.Len() != 0 {
		t.Fatalf("empty TSV = %q, want empty", tsv.String())
	}

	var jsonOutput bytes.Buffer
	if err := app.ListWithFormat(&jsonOutput, ListFormatJSON); err != nil {
		t.Fatal(err)
	}
	if got, want := jsonOutput.String(), "[]\n"; got != want {
		t.Fatalf("empty JSON = %q, want %q", got, want)
	}
}

func TestParseListFormat(t *testing.T) {
	t.Parallel()

	for _, format := range []ListFormat{ListFormatTable, ListFormatTSV, ListFormatJSON} {
		if got, err := ParseListFormat(string(format)); err != nil || got != format {
			t.Fatalf("ParseListFormat(%q) = %q, %v", format, got, err)
		}
	}
	if got, err := ParseListFormat(" JSON "); err != nil || got != ListFormatJSON {
		t.Fatalf("ParseListFormat( JSON ) = %q, %v", got, err)
	}
	if _, err := ParseListFormat("yaml"); err == nil || !strings.Contains(err.Error(), "choose table, tsv, or json") {
		t.Fatalf("ParseListFormat(yaml) error = %v", err)
	}
}

func TestListWithFormatNormalizesFormatBeforeDispatch(t *testing.T) {
	t.Parallel()

	app := New(t.TempDir(), &fakeCloner{contents: map[string]string{
		"https://example.com/acme/alpha.git": "alpha source",
	}})
	if err := app.Add(context.Background(), "https://example.com/acme/alpha.git"); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := app.ListWithFormat(&output, ListFormat(" JSON ")); err != nil {
		t.Fatalf("ListWithFormat() error = %v", err)
	}
	if !strings.Contains(output.String(), `"name":"alpha"`) {
		t.Fatalf("normalized JSON output = %q", output.String())
	}
}
