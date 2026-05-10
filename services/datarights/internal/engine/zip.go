package engine

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
)

// CSVFile represents a single CSV file to include in the export ZIP.
type CSVFile struct {
	Name    string     // Filename within the ZIP (e.g., "expenses.csv")
	Headers []string
	Rows    [][]string
}

// BuildZIP creates an in-memory ZIP archive from the given CSV files.
func BuildZIP(files []CSVFile) ([]byte, error) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	for _, file := range files {
		fw, err := zw.Create(file.Name)
		if err != nil {
			return nil, err
		}

		cw := csv.NewWriter(fw)
		if err := cw.Write(file.Headers); err != nil {
			return nil, err
		}
		if err := cw.WriteAll(file.Rows); err != nil {
			return nil, err
		}
		cw.Flush()

		if err := cw.Error(); err != nil {
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
