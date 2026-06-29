package main

import (
	"fmt"
	"log"

	"github.com/mgierada/calculon/internal/data_processing/parsers"
	"github.com/mgierada/calculon/internal/data_processing/readers"
)

func main() {
	xlsxFile := "/Users/maciej.gierada/Desktop/account_50747414_pl_xlsx_2026-05-27_2026-06-27/account_50747414_pl_xlsx_2026-05-27_2026-06-27.xlsx"
	// csvFile := "/Users/maciej.gierada/Downloads/EAFifa.csv"
	// content := reader.ReadCsvFile(csvFile)
	content, err := reader.ReadXlsx(xlsxFile, "CLOSED POSITION HISTORY")
	if err != nil {
		log.Fatal(err)
	}

	positions, err := parsers.ParseClosedPositions(content)
	if err != nil {
		log.Fatal(err)
	}

	for _, p := range positions {
		fmt.Printf("%+v\n", p)
	}
}
