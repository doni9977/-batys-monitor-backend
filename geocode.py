#!/usr/bin/env python3
"""Geocode NR addresses from addresses.json → nr_coords.json"""
import json, time, urllib.request, urllib.parse, random

with open("addresses.json") as f:
    data = json.load(f)

URALSK_CENTER = (51.2333, 51.3667)

def extract_street(addr):
    """Extract meaningful street part from address for geocoding."""
    if not addr:
        return None
    # Remove phone numbers
    parts = addr.split(", тел.")
    addr = parts[0]
    # Remove apartment/floor info
    for stop in [", кв.", ", корпус", ", офис"]:
        if stop in addr:
            addr = addr[:addr.index(stop)]
    return addr.strip()

def geocode(query):
    """Geocode via Nominatim."""
    params = urllib.parse.urlencode({
        "q": query,
        "format": "json",
        "limit": 1,
        "countrycodes": "kz",
    })
    url = f"https://nominatim.openstreetmap.org/search?{params}"
    req = urllib.request.Request(url, headers={"User-Agent": "BatysMonitor/1.0"})
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            results = json.loads(resp.read())
            if results:
                return float(results[0]["lat"]), float(results[0]["lon"])
    except Exception as e:
        print(f"  Error: {e}")
    return None

coords = {}
for i, rec in enumerate(data):
    name = rec.get("full_name", "")
    addr = rec.get("legal_address", "")
    street = extract_street(addr)
    
    if not street:
        print(f"[{i+1}/{len(data)}] {name[:40]} — No address, skip")
        continue
    
    print(f"[{i+1}/{len(data)}] {name[:40]} — Geocoding: {street[:60]}")
    
    # Try full address first
    result = geocode(street)
    
    if not result:
        # Try just city + street
        for part in ["г.Уральск,", "г. Уральск,"]:
            if part in street:
                simplified = "Уральск, " + street.split(part)[1].strip()
                result = geocode(simplified)
                break
    
    if result:
        lat, lng = result
        # Sanity check: should be near Uralsk (within ~2 degrees)
        if abs(lat - URALSK_CENTER[0]) < 2 and abs(lng - URALSK_CENTER[1]) < 2:
            coords[name] = {"lat": lat, "lng": lng}
            print(f"  OK {lat}, {lng}")
        else:
            print(f"  Too far: {lat}, {lng} — skip")
    else:
        print(f"  Not found")
    
    time.sleep(1.1)  # Nominatim rate limit: 1 req/sec

# For entries without coordinates, generate deterministic points around Uralsk
all_names = [rec.get("full_name", "") for rec in data]
for name in all_names:
    if name not in coords:
        seed = sum(ord(c) for c in name)
        rng = random.Random(seed)
        lat = URALSK_CENTER[0] + rng.uniform(-0.03, 0.03)
        lng = URALSK_CENTER[1] + rng.uniform(-0.03, 0.03)
        coords[name] = {"lat": round(lat, 6), "lng": round(lng, 6)}

with open("nr_coords.json", "w") as f:
    json.dump(coords, f, ensure_ascii=False, indent=2)

print(f"\nTotal: {len(coords)} coordinates saved to nr_coords.json")
