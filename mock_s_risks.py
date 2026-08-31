import openpyxl
from datetime import datetime, timedelta

file_path = '../Пролеченные случаи стационара/2026 Талап.xlsx'
wb = openpyxl.load_workbook(file_path)
ws = wb.active

patients = [
    ("А.А.А.", "01.01.1960", datetime(2023, 7, 3)),
    ("А.А.А.", "01.02.2017", datetime(2023, 7, 26)),
    ("А.А.А.", "01.04.1981", datetime(2023, 8, 15)),
    ("А.А.А.", "01.07.2022", datetime(2023, 11, 20)),
    ("А.А.А.", "01.09.2003", datetime(2023, 10, 4)),
    ("А.А.А.", "03.05.1998", datetime(2023, 12, 28)),
    ("А.А.А.", "03.08.2003", datetime(2023, 4, 5)),
    ("А.А.А.", "04.07.1988", datetime(2023, 1, 19)),
    ("А.А.А.", "04.07.2003", datetime(2023, 2, 8)),
    ("А.А.А.", "04.09.2006", datetime(2023, 9, 11)),
]

start_row = 10

# S1: 5 cases (overlapping)
for i in range(5):
    row = start_row + i
    p_name, p_dob, s_date = patients[i]
    ws.cell(row=row, column=3).value = p_name
    ws.cell(row=row, column=4).value = p_dob
    ws.cell(row=row, column=6).value = s_date - timedelta(days=2)
    ws.cell(row=row, column=8).value = s_date + timedelta(days=2)
    ws.cell(row=row, column=31).value = "Врач-КроссЧек"

# S5: 5 cases (Died before service_date)
for i in range(5, 10):
    row = start_row + i
    p_name, p_dob, s_date = patients[i]
    ws.cell(row=row, column=3).value = p_name
    ws.cell(row=row, column=4).value = p_dob
    ws.cell(row=row, column=6).value = s_date - timedelta(days=5)
    ws.cell(row=row, column=8).value = s_date - timedelta(days=2)
    ws.cell(row=row, column=12).value = "Умер"
    ws.cell(row=row, column=31).value = "Врач-МертвыеДуши"

# S4: 11 cases for Doctor "Экстренный Врач"
s4_start = start_row + 10
for i in range(15):
    row = s4_start + i
    ws.cell(row=row, column=3).value = f"Пациент {i}"
    ws.cell(row=row, column=31).value = "Экстренный Врач"
    ws.cell(row=row, column=29).value = "Отделение хирургии"
    if i < 14:
        ws.cell(row=row, column=16).value = 1 # Экстренно
        ws.cell(row=row, column=15).value = 0 # Планово
    else:
        ws.cell(row=row, column=16).value = 0
        ws.cell(row=row, column=15).value = 1

wb.save(file_path)
