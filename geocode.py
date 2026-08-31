import psycopg2
from geopy.geocoders import Nominatim
from geopy.extra.rate_limiter import RateLimiter
import json
import time

conn = psycopg2.connect("dbname=batys_monitor user=batys_user password=batys_password host=localhost port=5440")
cur = conn.cursor()

cur.execute("SELECT DISTINCT clinic_name FROM detected_risks;")
clinics = [row[0] for row in cur.fetchall()]

geolocator = Nominatim(user_agent="batys_monitor_app")
geocode = RateLimiter(geolocator.geocode, min_delay_seconds=1)

results = {}

print("Starting geocoding...")
for name in clinics:
    # Try to find address in nr_records
    cur.execute("SELECT legal_address FROM nr_records WHERE full_name = %s LIMIT 1", (name,))
    addr_row = cur.fetchone()
    
    query = ""
    if addr_row and addr_row[0]:
        addr = addr_row[0]
        # Clean up address a bit for Nominatim
        # Keep "Уральск" and street
        addr = addr.replace("Западно-Казахстанская область,", "").replace("ЗКО,", "")
        query = addr + ", Казахстан"
    else:
        query = name + ", Уральск, Казахстан"
        
    print(f"Geocoding {name} -> {query}")
    try:
        location = geocode(query, timeout=10)
        if location:
            results[name] = {"lat": location.latitude, "lng": location.longitude}
            print(f"  FOUND: {location.latitude}, {location.longitude}")
        else:
            # Fallback to Uralsk center with slight jitter
            print("  NOT FOUND")
    except Exception as e:
        print("  ERROR:", e)

with open('coords.json', 'w') as f:
    json.dump(results, f, ensure_ascii=False, indent=2)

print("Done. Dumped to coords.json")
