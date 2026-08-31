import psycopg2
import pandas as pd
from datetime import timedelta

# Connect to DB
conn = psycopg2.connect("dbname=batys user=batys_user password=batys_pass host=localhost port=5440")
cur = conn.cursor()

# Get 5 outpatient services for S1 (overlap)
cur.execute("SELECT patient_name, service_date, patient_dob FROM service_records WHERE patient_name != '' AND patient_name IS NOT NULL LIMIT 5")
s1_patients = cur.fetchall()

# Get 5 outpatient services for S5 (after death)
cur.execute("SELECT patient_name, service_date, patient_dob FROM service_records WHERE patient_name != '' AND patient_name IS NOT NULL OFFSET 5 LIMIT 5")
s5_patients = cur.fetchall()

print("S1 targets:", s1_patients)
print("S5 targets:", s5_patients)
