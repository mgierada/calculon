package reader

import (
	"fmt"
	"log"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ReadXlsx returns the rows of a single sheet.
func ReadXlsx(filePath, sheet string) ([][]string, error) {
	file, err := open(filePath)
	if err != nil {
		return nil, err
	}
	defer closeFile(file)

	rows, err := file.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet %q in %q: %w", sheet, filePath, err)
	}

	return rows, nil
}

// XlsxSheetNames lists the workbook's sheet names, trimmed, without reading any
// cells. It is cheap enough to sniff a file's format.
func XlsxSheetNames(filePath string) ([]string, error) {
	file, err := open(filePath)
	if err != nil {
		return nil, err
	}
	defer closeFile(file)

	names := file.GetSheetList()
	for i, name := range names {
		names[i] = strings.TrimSpace(name)
	}
	return names, nil
}

// ReadXlsxSheets returns every sheet in the workbook keyed by its name, opening
// the file once. Sheet names are trimmed because providers are inconsistent
// about trailing whitespace.
func ReadXlsxSheets(filePath string) (map[string][][]string, error) {
	file, err := open(filePath)
	if err != nil {
		return nil, err
	}
	defer closeFile(file)

	names := file.GetSheetList()
	sheets := make(map[string][][]string, len(names))
	for _, name := range names {
		rows, err := file.GetRows(name)
		if err != nil {
			return nil, fmt.Errorf("failed to read sheet %q in %q: %w", name, filePath, err)
		}
		sheets[strings.TrimSpace(name)] = rows
	}

	return sheets, nil
}

func open(filePath string) (*excelize.File, error) {
	file, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open xlsx file at %q: %w", filePath, err)
	}
	return file, nil
}

func closeFile(file *excelize.File) {
	if err := file.Close(); err != nil {
		log.Println(err)
	}
}
