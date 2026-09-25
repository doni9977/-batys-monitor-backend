import re

with open('internal/parser/excel_parser.go', 'r') as f:
    content = f.read()

# Fix patientNameIdx to avoid "id"
# Currently: patientNameIdx := findColIndex("пациенты", "пациент", "фиофизлица", "фио")
# We can change it to a custom loop or just add a negative check in findColIndex.
# But it's easier to change the keywords to exact matches or prioritize "фио".

replacement = """	patientNameIdx := findColIndex("фиофизлица", "фио", "пациенты")
	// Если не нашли, ищем "пациент", но избегаем "id"
	if patientNameIdx == -1 {
		for idx, colName := range columnNames {
			if strings.Contains(colName, "пациент") && !strings.Contains(colName, "id") {
				patientNameIdx = idx
				break
			}
		}
	}"""

content = re.sub(r'patientNameIdx := findColIndex\("пациенты", "пациент", "фиофизлица", "фио"\)', replacement, content)

with open('internal/parser/excel_parser.go', 'w') as f:
    f.write(content)
