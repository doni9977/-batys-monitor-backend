#!/usr/bin/env python3
"""Геокодер нерезидентов (ТОО) через 2GIS API — ищет по юр. адресу."""
import json, urllib.request, urllib.parse, time, os, subprocess

API_KEY = "fc827f62-089b-4be6-879b-bc09ec558253"
URALSK_CENTER = (51.2333, 51.3667)

def geocode_address_2gis(address, name):
    """Ищет по юридическому адресу через 2GIS Geocoder."""
    # Очищаем адрес от лишнего
    clean_addr = address.split(", тел.")[0]
    for stop in [", кв.", ", корпус", ", офис", "кв."]:
        if stop in clean_addr:
            clean_addr = clean_addr[:clean_addr.index(stop)]

    params = urllib.parse.urlencode({
        "q": clean_addr.strip(),
        "key": API_KEY,
        "fields": "items.point,items.full_name,items.address_name",
        "locale": "ru_KZ",
        "page_size": 1
    })
    url = f"https://catalog.api.2gis.com/3.0/items?{params}"
    req = urllib.request.Request(url, headers={"User-Agent": "BatysMonitor/1.0"})

    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            data = json.loads(resp.read())
            items = data.get("result", {}).get("items", [])
            if items:
                point = items[0].get("point", {})
                lat = point.get("lat")
                lon = point.get("lon")
                found_name = items[0].get("name", "")
                found_addr = items[0].get("address_name", "")
                full_address = f"{found_name}, {found_addr}" if found_addr else found_name
                if lat and lon:
                    return float(lat), float(lon), full_address
    except Exception as e:
        print(f"  [ОШИБКА] {e}")

    return None

def main():
    # Получаем список нерезидентов (название и юр. адрес)
    cmd = """docker exec batys-monitor-backend-postgres-1 psql -U batys_user -d batys_monitor -t -A -F"|" -c "SELECT DISTINCT full_name, legal_address FROM nr_records WHERE legal_address IS NOT NULL AND legal_address <> '';" """
    result = subprocess.run(cmd, shell=True, capture_output=True, text=True)

    records = []
    for line in result.stdout.split('\n'):
        if line.strip() and "|" in line:
            name, addr = line.split("|", 1)
            records.append((name.strip(), addr.strip()))

    coords_file = "../batys-front/src/assets/nr_coords_yandex.json"
    coords = {}
    if os.path.exists(coords_file):
        with open(coords_file, 'r', encoding='utf-8') as f:
            coords = json.load(f)

    updated = False
    for name, addr in records:
        if name not in coords or coords[name].get("lat") == URALSK_CENTER[0]:
            print(f"Поиск: {name[:40]}... ({addr[:40]})")
            res = geocode_address_2gis(addr, name)
            if res:
                lat, lng, found_addr = res
                coords[name] = {"lat": lat, "lng": lng, "address": found_addr}
                print(f"  ✅ {found_addr} ({lat}, {lng})")
                updated = True
            else:
                print("  ❌ Не найдено.")
                coords[name] = {"lat": URALSK_CENTER[0], "lng": URALSK_CENTER[1], "address": addr}
                updated = True
            time.sleep(0.3)

    if updated or not os.path.exists(coords_file):
        os.makedirs(os.path.dirname(coords_file), exist_ok=True)
        with open(coords_file, 'w', encoding='utf-8') as f:
            json.dump(coords, f, ensure_ascii=False, indent=2)
        print(f"\nСохранено {len(coords)} нерезидентов в {coords_file}")
    else:
        print("\nНовых нерезидентов нет. Координаты актуальны.")

if __name__ == "__main__":
    main()
