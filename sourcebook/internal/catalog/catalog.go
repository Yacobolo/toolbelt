package catalog

import (
	"fmt"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/providers/datastar"
	"github.com/Yacobolo/toolbelt/sourcebook/internal/providers/netsuite"
	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"
)

const (
	AzureDocsID    = "azure-docs"
	DBTDocsID      = "dbt-docs"
	DuckDBDocsID   = "duckdb-docs"
	DuckLakeDocsID = "ducklake-docs"
	PowerBIDocsID  = "powerbi-docs"
)

// Register adds every built-in retrieval provider and selectable source preset.
func Register(app *sourcebook.App) error {
	for _, definition := range []sourcebook.ProviderDefinition{
		datastar.Definition(),
		netsuite.Definition(),
	} {
		if err := app.RegisterProvider(definition); err != nil {
			return fmt.Errorf("register provider %q: %w", definition.ID, err)
		}
	}

	for _, entry := range []sourcebook.CatalogEntry{
		datastar.CatalogEntry(),
		netsuite.CatalogEntry(),
		azureDocs(),
		dbtDocs(),
		duckDBDocs(),
		duckLakeDocs(),
		powerBIDocs(),
	} {
		if err := app.RegisterCatalogEntry(entry); err != nil {
			return fmt.Errorf("register catalogue preset %q: %w", entry.ID, err)
		}
	}
	return nil
}

func azureDocs() sourcebook.CatalogEntry {
	return gitDocs(
		AzureDocsID,
		"Azure documentation",
		"Official Microsoft Azure documentation",
		"https://github.com/MicrosoftDocs/azure-docs.git",
		"main",
		"articles",
	)
}

func dbtDocs() sourcebook.CatalogEntry {
	return gitDocs(
		DBTDocsID,
		"dbt documentation",
		"Official dbt product documentation",
		"https://github.com/dbt-labs/docs.getdbt.com.git",
		"current",
		"website/docs",
	)
}

func duckDBDocs() sourcebook.CatalogEntry {
	return gitDocs(
		DuckDBDocsID,
		"DuckDB documentation",
		"Official DuckDB documentation",
		"https://github.com/duckdb/duckdb-web.git",
		"main",
		"docs",
	)
}

func duckLakeDocs() sourcebook.CatalogEntry {
	return gitDocs(
		DuckLakeDocsID,
		"DuckLake documentation",
		"Official DuckLake documentation and specification",
		"https://github.com/duckdb/ducklake-web.git",
		"main",
		"docs",
	)
}

func powerBIDocs() sourcebook.CatalogEntry {
	return gitDocs(
		PowerBIDocsID,
		"Power BI documentation",
		"Official Microsoft Power BI documentation",
		"https://github.com/MicrosoftDocs/powerbi-docs.git",
		"main",
		"powerbi-docs",
	)
}

func gitDocs(id, displayName, description, repositoryURL, ref, root string) sourcebook.CatalogEntry {
	return sourcebook.CatalogEntry{
		ID:          id,
		DisplayName: displayName,
		Description: description,
		Provider:    sourcebook.ProviderGit,
		SourceName:  id,
		SourceURL:   repositoryURL,
		GitRef:      ref,
		GitRoot:     root,
		GitTextOnly: true,
	}
}
