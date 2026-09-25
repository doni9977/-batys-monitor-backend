import re

with open('internal/services/risk_engine.go', 'r') as f:
    content = f.read()

# Add PatientName string to structs that have PatientIIN
content = re.sub(r'(PatientIIN\s+string)', r'\1\n\t\tPatientName    string', content)

# Add sr.patient_name to SQL queries that have sr.patient_iin
content = re.sub(r'sr\.patient_iin,', r'sr.patient_iin,\n            sr.patient_name,', content)

# Add sr.patient_name to GROUP BY if needed (for A4, A7)
content = re.sub(r'GROUP BY sr\.doctor_name, sr\.patient_iin,', r'GROUP BY sr.doctor_name, sr.patient_iin, sr.patient_name,', content)

# Add "patient_name" to details map
content = re.sub(r'("patient_iin":\s+r\.PatientIIN,)', r'\1\n\t\t\t"patient_name":      r.PatientName,', content)

with open('internal/services/risk_engine.go', 'w') as f:
    f.write(content)

