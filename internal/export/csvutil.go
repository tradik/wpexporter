package export

// csvSafe neutralizes CSV formula injection. Spreadsheet applications
// (Excel, LibreOffice, Google Sheets) interpret a cell whose first character is
// '=', '+', '-', '@', a tab or a carriage return as a formula. Since exported
// values originate from untrusted WordPress content (titles, tags, author names),
// such a value could execute a formula when the merchant opens the CSV. Prefixing
// the value with a single quote forces the spreadsheet to treat it as text (INT-001).
func csvSafe(v string) string {
	if v == "" {
		return v
	}
	switch v[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + v
	}
	return v
}

// csvSafeRow returns a copy of row with every field passed through csvSafe.
// Use it for data rows before writing; header rows are trusted constants.
func csvSafeRow(row []string) []string {
	out := make([]string, len(row))
	for i, v := range row {
		out[i] = csvSafe(v)
	}
	return out
}
