import re

headers = ["Пациент", "Цена", "ФИОФизлица", "Дата рождения", "ИИН", "ID пациента", "Какой-то номер"]

def normalize_header(h):
    return re.sub(r'[^a-zA-Zа-яА-ЯёЁ0-9]', '', h).lower()

column_names = [normalize_header(h) for h in headers]
print("Normalized:", column_names)

def find_col_index(*keywords):
    for kw in keywords:
        kw_lower = normalize_header(kw)
        for i, col in enumerate(column_names):
            if kw_lower in col:
                return i
    return -1

idx = find_col_index("фиофизлица", "фио", "пациенты")
if idx == -1:
    for i, col in enumerate(column_names):
        if "пациент" in col and "id" not in col:
            idx = i
            break
print("Found index:", idx)
if idx != -1:
    print("Column name:", headers[idx])
