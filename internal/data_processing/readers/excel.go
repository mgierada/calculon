package reader

import (
	"fmt"
	"log"

	"github.com/xuri/excelize/v2"
)

func ReadXlsx(filePath, sheet string) ([][]string, error) {
	file, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open xlsx file at %q: %w", filePath, err)
	}

	defer func() {
		if err := file.Close(); err != nil {
			log.Println(err)
		}
	}()

	rows, err := file.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet %q in %q: %w", sheet, filePath, err)
	}

	return rows, nil
}
