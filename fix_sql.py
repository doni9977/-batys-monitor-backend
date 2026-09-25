import re

with open('internal/services/risk_engine.go', 'r') as f:
    content = f.read()

# Fix final SELECTs where patient_iin is used without sr.
content = re.sub(r'(\s+)patient_iin,', r'\1patient_iin,\n\1patient_name,', content)

# Check if collectA8 is missing patient_name
# A8 doesn't use CTE, it uses sr.patient_iin, so it was updated correctly.
# But let's check A1 and A2.

with open('internal/services/risk_engine.go', 'w') as f:
    f.write(content)
