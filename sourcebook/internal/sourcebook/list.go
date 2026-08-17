package sourcebook

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"
)

// ListFormat controls how sourcebook list renders its sources.
type ListFormat string

const (
	ListFormatTable ListFormat = "table"
	ListFormatTSV   ListFormat = "tsv"
	ListFormatJSON  ListFormat = "json"
)

// ParseListFormat validates a list output format supplied by a command-line
// user.
func ParseListFormat(value string) (ListFormat, error) {
	format := ListFormat(strings.ToLower(strings.TrimSpace(value)))
	switch format {
	case ListFormatTable, ListFormatTSV, ListFormatJSON:
		return format, nil
	default:
		return "", fmt.Errorf("invalid list format %q; choose table, tsv, or json", value)
	}
}

// List renders sources in the stable, tab-separated format used by previous
// Sourcebook releases.
func (a *App) List(output io.Writer) error {
	return a.ListWithFormat(output, ListFormatTSV)
}

// ListWithFormat renders sources in a human-readable table, tab-separated
// records, or JSON.
func (a *App) ListWithFormat(output io.Writer, format ListFormat) error {
	parsedFormat, err := ParseListFormat(string(format))
	if err != nil {
		return err
	}
	sources, err := a.SourcesWithSizes()
	if err != nil {
		return err
	}

	switch parsedFormat {
	case ListFormatTable:
		return writeListTable(output, sources)
	case ListFormatTSV:
		return writeListTSV(output, sources)
	case ListFormatJSON:
		return writeListJSON(output, sources)
	default:
		panic("validated list format")
	}
}

func writeListTable(output io.Writer, sources []Source) error {
	if len(sources) == 0 {
		_, err := fmt.Fprintln(output, "No sources configured.")
		return err
	}

	table := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "NAME\tPROVIDER\tUPDATED\tSIZE\tURL"); err != nil {
		return fmt.Errorf("write source list: %w", err)
	}
	for _, source := range sources {
		if _, err := fmt.Fprintf(
			table,
			"%s\t%s\t%s\t%s\t%s\n",
			source.Name,
			source.Provider,
			listUpdatedAt(source),
			listSize(source.SizeBytes),
			source.DisplayURL(),
		); err != nil {
			return fmt.Errorf("write source list: %w", err)
		}
	}
	if err := table.Flush(); err != nil {
		return fmt.Errorf("write source list: %w", err)
	}
	return nil
}

func writeListTSV(output io.Writer, sources []Source) error {
	for _, source := range sources {
		sizeBytes := "unknown"
		if source.SizeBytes != nil {
			sizeBytes = fmt.Sprintf("%d", *source.SizeBytes)
		}
		if _, err := fmt.Fprintf(
			output,
			"%s\t%s\t%s\t%s\t%s\n",
			source.Name,
			source.Provider,
			source.DisplayURL(),
			listUpdatedAt(source),
			sizeBytes,
		); err != nil {
			return fmt.Errorf("write source list: %w", err)
		}
	}
	return nil
}

type listJSONSource struct {
	Name      string     `json:"name"`
	Provider  string     `json:"provider"`
	URL       string     `json:"url"`
	UpdatedAt *time.Time `json:"updated_at"`
	SizeBytes *int64     `json:"size_bytes"`
}

func writeListJSON(output io.Writer, sources []Source) error {
	records := make([]listJSONSource, 0, len(sources))
	for _, source := range sources {
		record := listJSONSource{
			Name:      source.Name,
			Provider:  source.Provider,
			URL:       source.DisplayURL(),
			SizeBytes: source.SizeBytes,
		}
		if !source.UpdatedAt.IsZero() {
			updatedAt := source.UpdatedAt.UTC()
			record.UpdatedAt = &updatedAt
		}
		records = append(records, record)
	}
	if err := json.NewEncoder(output).Encode(records); err != nil {
		return fmt.Errorf("write source list: %w", err)
	}
	return nil
}

func listUpdatedAt(source Source) string {
	if source.UpdatedAt.IsZero() {
		return "never"
	}
	return source.UpdatedAt.UTC().Format(time.RFC3339)
}

func listSize(sizeBytes *int64) string {
	if sizeBytes == nil {
		return "unknown"
	}
	const unit = 1024
	if *sizeBytes < unit {
		return fmt.Sprintf("%d B", *sizeBytes)
	}
	size := float64(*sizeBytes)
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	for _, suffix := range units {
		size /= unit
		if size < unit || suffix == units[len(units)-1] {
			return fmt.Sprintf("%.1f %s", size, suffix)
		}
	}
	return "unknown"
}
