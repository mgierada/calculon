package reader

import (
	"encoding/csv"
	"fmt"
	"os"
)

// ReadCsvFile returns every record of a CSV file. Rows are allowed to have a
// varying field count so provider exports with ragged headers still parse.
func ReadCsvFile(filePath string) ([][]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open csv file at %q: %w", filePath, err)
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	csvReader.FieldsPerRecord = -1

	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse %q as csv: %w", filePath, err)
	}

	return records, nil
}
