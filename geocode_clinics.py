#!/usr/bin/env python3
import json, urllib.request, urllib.parse, time, os, subprocess

URALSK_CENTER = (51.2333, 51.3667)

def geocode(query):
    params = urllib.parse.urlencode({
        "q": query + ", Уральск, Казахстан",
        "format": "json",
        "limit": 1
    })
    url = f"https://nominatim.openstreetmap.org/search?{params}"
    req = urllib.request.Request(url, headers={"User-Agent": "BatysMonitor/1.0"})
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            results = json.loads(resp.read())
            if results:
                return float(results[0]["lat"]), float(results[0]["lon"]), results[0].get("display_name", "")
    except Exception as e:
        print(f"Error geocoding {query}: {e}")
    return None

def main():
    # 1. Fetch clinics from DB
    cmd = """docker exec batys-monitor-backend-postgres-1 psql -U batys_user -d batys_monitor -t -A -c "SELECT DISTINCT clinic_name FROM service_records UNION SELECT DISTINCT hospital_name as clinic_name FROM inpatient_records;" """
    result = subprocess.run(cmd, shell=True, capture_output=True, text=True)
    clinics = [line.strip() for line in result.stdout.split('\n') if line.strip()]
    
    # 2. Load existing coords from a JSON file (if any) to avoid re-geocoding
    coords_file = "../batys-front/src/assets/clinic_coords.json"
    coords = {}
    if os.path.exists(coords_file):
        with open(coords_file, 'r', encoding='utf-8') as f:
            coords = json.load(f)
            
    updated = False
    for clinic in clinics:
        if clinic not in coords:
            print(f"Geocoding: {clinic}...")
            res = geocode(clinic)
            if res:
                lat, lng, address = res
                coords[clinic] = {"lat": lat, "lng": lng, "address": address}
                print(f"  Found: {address}")
            else:
                print("  Not found. Using default Uralsk center with slight offset.")
                # fallback
                coords[clinic] = {"lat": URALSK_CENTER[0], "lng": URALSK_CENTER[1], "address": "г. Уральск (Координаты уточняются)"}
            updated = True
            time.sleep(1)
            
    if updated or not os.path.exists(coords_file):
        os.makedirs(os.path.dirname(coords_file), exist_ok=True)
        with open(coords_file, 'w', encoding='utf-8') as f:
            json.dump(coords, f, ensure_ascii=False, indent=2)
        print(f"Saved {len(coords)} clinics to {coords_file}")
    else:
        print("No new clinics to geocode.")

if __name__ == "__main__":
    main()
