import pandas as pd
import sys

def dump(file):
    print(f"\n--- {file} ---")
    try:
        df = pd.read_excel(file, header=6) # 6 corresponds to 7th row
        print(df.head(2).to_dict(orient='records'))
    except Exception as e:
        print("Error:", e)

dump("таблица за 2025-2026 года (1).xlsx")
dump("Пролеченные случаи стационара/2026 Талап.xlsx")
