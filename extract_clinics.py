import subprocess
import json

# Command to get unique clinic names from both tables
cmd = """docker exec batys-monitor-backend-postgres-1 psql -U batys_user -d batys_monitor -t -A -c "
SELECT DISTINCT clinic_name FROM service_records
UNION
SELECT DISTINCT hospital_name as clinic_name FROM inpatient_records;
" """

result = subprocess.run(cmd, shell=True, capture_output=True, text=True)
clinics = [line.strip() for line in result.stdout.split('\n') if line.strip()]

with open('clinics.json', 'w', encoding='utf-8') as f:
    json.dump(clinics, f, ensure_ascii=False, indent=2)

print(f"Extracted {len(clinics)} clinics.")
