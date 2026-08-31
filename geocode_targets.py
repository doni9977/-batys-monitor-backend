import json
import urllib.request
import urllib.parse
import time

with open("targets.json", "r") as f:
    targets = json.load(f)

results = {}

print(f"Loaded {len(targets)} targets.")

for t in targets:
    name = t["name"]
    addr = t["address"]
    
    # Heuristics for query
    query = ""
    if addr:
        clean_addr = addr.replace("Западно-Казахстанская область,", "").replace("ЗКО,", "")
        query = clean_addr + ", Казахстан"
    else:
        # For clinics, they are in Uralsk
        clean_name = name.replace('Филиал по Западно-Казахстанской области НАО «ФСМС»', 'Фонд медицинского страхования')
        clean_name = clean_name.replace('Акционерное общество "Талап"', 'Талап')
        clean_name = clean_name.replace('ГКП "Городская поликлиника №2"', 'Городская поликлиника №2')
        clean_name = clean_name.replace('ГКП на праве хозяйственного ведения "Городская поликлиника №1" управления здравоохранения акимата Западно-Казахстанской области', 'Городская поликлиника №1')
        clean_name = clean_name.replace('ТОО "Uniserv Medical Center"', 'Uniserv Medical Center')
        
        query = clean_name + ", Уральск, Казахстан"
    
    print(f"Searching: {query}")
    url = f"https://nominatim.openstreetmap.org/search?q={urllib.parse.quote(query)}&format=json&limit=1"
    
    req = urllib.request.Request(url, headers={'User-Agent': 'batys-monitor/1.0'})
    
    try:
        with urllib.request.urlopen(req, timeout=5) as response:
            data = json.loads(response.read().decode())
            if data:
                lat = float(data[0]['lat'])
                lon = float(data[0]['lon'])
                results[name] = {"lat": lat, "lng": lon}
                print(f"  -> Found: {lat}, {lon}")
            else:
                print("  -> Not found")
    except Exception as e:
        print(f"  -> Error: {e}")
        
    time.sleep(1) # Be nice to Nominatim

with open("coords.json", "w") as f:
    json.dump(results, f, ensure_ascii=False, indent=2)
print("Done. Saved to coords.json")
