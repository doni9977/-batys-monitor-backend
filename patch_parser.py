import re

with open('internal/parser/excel_parser.go', 'r') as f:
    content = f.read()

# Fix patientNameIdx logic to also ignore "идентификатор"
old_logic = """	patientNameIdx := findColIndex("фиофизлица", "фио", "пациенты")
	// Если не нашли, ищем "пациент", но избегаем "id"
	if patientNameIdx == -1 {
		for idx, colName := range columnNames {
			if strings.Contains(colName, "пациент") && !strings.Contains(colName, "id") {
				patientNameIdx = idx
				break
			}
		}
	}"""

new_logic = """	patientNameIdx := findColIndex("фиофизлица", "фио", "пациенты")
	// Если не нашли, ищем "пациент", но избегаем "id" и "идентификатор"
	if patientNameIdx == -1 {
		for idx, colName := range columnNames {
			if strings.Contains(colName, "пациент") && !strings.Contains(colName, "id") && !strings.Contains(colName, "идентификатор") {
				patientNameIdx = idx
				break
			}
		}
	}"""

content = content.replace(old_logic, new_logic)

with open('internal/parser/excel_parser.go', 'w') as f:
    f.write(content)

