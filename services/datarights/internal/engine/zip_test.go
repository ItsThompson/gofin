package engine

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildZIP_SingleFile(t *testing.T) {
	files := []CSVFile{
		{
			Name:    "profile.csv",
			Headers: []string{"username", "email", "currency", "role", "account_created_at"},
			Rows:    [][]string{{"alex", "alex@example.com", "USD", "user", "2025-03-15T10:30:00Z"}},
		},
	}

	zipBytes, err := BuildZIP(files)
	require.NoError(t, err)
	assert.NotEmpty(t, zipBytes)

	// Verify the ZIP is valid and contains the expected file
	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	require.NoError(t, err)
	require.Len(t, reader.File, 1)
	assert.Equal(t, "profile.csv", reader.File[0].Name)

	// Read and verify CSV content
	f, err := reader.File[0].Open()
	require.NoError(t, err)
	defer f.Close()

	csvReader := csv.NewReader(f)
	records, err := csvReader.ReadAll()
	require.NoError(t, err)

	require.Len(t, records, 2) // header + 1 data row
	assert.Equal(t, []string{"username", "email", "currency", "role", "account_created_at"}, records[0])
	assert.Equal(t, []string{"alex", "alex@example.com", "USD", "user", "2025-03-15T10:30:00Z"}, records[1])
}

func TestBuildZIP_MultipleFiles(t *testing.T) {
	files := []CSVFile{
		{
			Name:    "profile.csv",
			Headers: []string{"username", "email"},
			Rows:    [][]string{{"alex", "alex@example.com"}},
		},
		{
			Name:    "expenses.csv",
			Headers: []string{"id", "name", "amount"},
			Rows: [][]string{
				{"1", "Groceries", "45.99"},
				{"2", "Gas", "30.00"},
			},
		},
	}

	zipBytes, err := BuildZIP(files)
	require.NoError(t, err)

	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	require.NoError(t, err)
	require.Len(t, reader.File, 2)

	assert.Equal(t, "profile.csv", reader.File[0].Name)
	assert.Equal(t, "expenses.csv", reader.File[1].Name)

	// Verify expenses.csv has 3 rows (header + 2 data)
	f, err := reader.File[1].Open()
	require.NoError(t, err)
	defer f.Close()

	csvReader := csv.NewReader(f)
	records, err := csvReader.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 3)
}

func TestBuildZIP_EmptyRows(t *testing.T) {
	files := []CSVFile{
		{
			Name:    "tags.csv",
			Headers: []string{"id", "name"},
			Rows:    [][]string{},
		},
	}

	zipBytes, err := BuildZIP(files)
	require.NoError(t, err)

	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	require.NoError(t, err)
	require.Len(t, reader.File, 1)

	f, err := reader.File[0].Open()
	require.NoError(t, err)
	defer f.Close()

	content, err := io.ReadAll(f)
	require.NoError(t, err)

	// Should have only the header row
	csvReader := csv.NewReader(bytes.NewReader(content))
	records, err := csvReader.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 1) // headers only
	assert.Equal(t, []string{"id", "name"}, records[0])
}

func TestBuildZIP_EmptyFileList(t *testing.T) {
	zipBytes, err := BuildZIP([]CSVFile{})
	require.NoError(t, err)
	assert.NotEmpty(t, zipBytes)

	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	require.NoError(t, err)
	assert.Len(t, reader.File, 0)
}
