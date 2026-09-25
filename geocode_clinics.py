#!/usr/bin/env python3
"""Геокодер клиник (ОСМС + Стационары) через 2GIS API."""
import json, urllib.request, urllib.parse, time, os, subprocess

API_KEY = "fc827f62-089b-4be6-879b-bc09ec558253"
URALSK_CENTER = (51.2333, 51.3667)

def normalize_clinic_name(name):
    """Normalize legal forms and generic clinic words for alias matching."""
    name = name.lower()
    for phrase in (
        "товарищество с ограниченной ответственностью",
        "акционерное общество",
        "государственное коммунальное предприятие",
        "тоо", "ао", "гкп", "медицинский центр", "медицинская организация",
        "медцентр", "клиника", "поликлиника", "больница", "medical center",
        "clinic", "hospital",
    ):
        name = name.replace(phrase, "")
    return "".join(char for char in name if char.isalnum())

def geocode_2gis(query):
    """Ищет организацию по названию в Уральске через 2GIS."""
    search_query = query.replace('ГКП на праве хозяйственного ведения', '').replace('управления здравоохранения акимата Западно-Казахстанской области', '').strip()
    search_query += ", Уральск"

    params = urllib.parse.urlencode({
        "q": search_query,
        "key": API_KEY,
        "fields": "items.point,items.full_name,items.address_name",
        "type": "branch",
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
                name = items[0].get("name", "")
                address = items[0].get("address_name", "")
                full_address = f"{name}, {address}" if address else name
                if lat and lon:
                    return float(lat), float(lon), full_address
    except Exception as e:
        print(f"  [ОШИБКА] {e}")

    return None

def main():
    # 1. Получаем список уникальных клиник из БД
    cmd = """docker exec batys-monitor-backend-postgres-1 psql -U batys_user -d batys_monitor -t -A -c "SELECT DISTINCT clinic_name FROM service_records UNION SELECT DISTINCT hospital_name as clinic_name FROM inpatient_records;" """
    result = subprocess.run(cmd, shell=True, capture_output=True, text=True)
    clinics = [line.strip() for line in result.stdout.split('\n') if line.strip()]

    coords_file = "../batys-front/src/assets/clinic_coords.json"
    coords = {}
    if os.path.exists(coords_file):
        with open(coords_file, 'r', encoding='utf-8') as f:
            coords = json.load(f)

    updated = False
    for clinic in clinics:
        current = coords.get(clinic, {})
        if current.get("address") and not current["address"].startswith("г. Уральск ("):
            continue

        normalized = normalize_clinic_name(clinic)
        alias = next(
            (value for name, value in coords.items()
             if normalize_clinic_name(name) == normalized
             and value.get("address")
             and not value["address"].startswith("г. Уральск (")),
            None,
        )
        if alias:
            coords[clinic] = alias.copy()
            print(f"Совпадение названия: {clinic} → {alias['address']}")
            updated = True
            continue

        print(f"Поиск в 2GIS: {clinic}...")
        res = geocode_2gis(clinic)
        if res:
            lat, lng, address = res
            coords[clinic] = {"lat": lat, "lng": lng, "address": address}
            print(f"  ✅ {address} ({lat}, {lng})")
            updated = True
        else:
            # Не сохраняем выдуманную точку: неизвестный адрес останется без маркера.
            print("  ❌ 2GIS не подтвердил адрес; случайная точка не записана.")
        time.sleep(0.3)

    if updated or not os.path.exists(coords_file):
        os.makedirs(os.path.dirname(coords_file), exist_ok=True)
        with open(coords_file, 'w', encoding='utf-8') as f:
            json.dump(coords, f, ensure_ascii=False, indent=2)
        print(f"\nСохранено {len(coords)} клиник в {coords_file}")
    else:
        print("\nНовых клиник нет. Координаты актуальны.")

if __name__ == "__main__":
    main()
