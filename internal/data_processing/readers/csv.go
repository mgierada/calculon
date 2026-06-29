package reader

import (
	"encoding/csv"
	"log"
	"os"
)

func ReadCsvFile(filePath string) [][]string {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal("Unable to read input file "+filePath, err)
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	records, err := csvReader.ReadAll()
	if err != nil {
		log.Fatal("Unable to parse file as CSV for %q: %v"+filePath, err)
	}

	return records
}
