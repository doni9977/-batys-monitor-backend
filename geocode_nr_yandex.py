#!/usr/bin/env python3
import json, urllib.request, urllib.parse, time, os, subprocess

# Ключ для "JavaScript API и HTTP Геокодер" (1000 запросов бесплатно навсегда)
GEOCODER_API_KEY = "ВАШ_КЛЮЧ_ГЕОКОДЕРА_ЗДЕСЬ"
URALSK_CENTER = (51.2333, 51.3667)

def geocode_address_yandex(address, name):
    # Очищаем адрес от лишнего мусора (квартиры, телефоны)
    clean_addr = address.split(", тел.")[0]
    for stop in [", кв.", ", корпус", ", офис", "кв."]:
        if stop in clean_addr:
            clean_addr = clean_addr[:clean_addr.index(stop)]
            
    search_query = clean_addr.strip()
    
    params = urllib.parse.urlencode({
        "geocode": search_query,
        "format": "json",
        "results": 1,
        "apikey": GEOCODER_API_KEY
    })
    url = f"https://geocode-maps.yandex.ru/1.x/?{params}"
    req = urllib.request.Request(url, headers={"User-Agent": "BatysMonitor/1.0"})
    
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            data = json.loads(resp.read())
            # Геокодер возвращает данные в другой структуре
            collection = data.get("response", {}).get("GeoObjectCollection", {})
            featureMember = collection.get("featureMember", [])
            
            if featureMember:
                # В геокодере точка возвращается в виде строки "долгота широта"
                point_str = featureMember[0]["GeoObject"]["Point"]["pos"]
                lon_str, lat_str = point_str.split()
                found_address = featureMember[0]["GeoObject"]["metaDataProperty"]["GeocoderMetaData"]["text"]
                return float(lat_str), float(lon_str), found_address
    except urllib.error.HTTPError as e:
        if e.code == 403:
            print("  [ОШИБКА] Ключ Геокодера недействителен (или вы использовали ключ от поиска организаций).")
        else:
            print(f"  [ОШИБКА] Yandex Geocoder API HTTP Error {e.code}")
    except Exception as e:
        print(f"  [ОШИБКА] {e}")
        
    return None

def main():
    if GEOCODER_API_KEY == "ВАШ_КЛЮЧ_ГЕОКОДЕРА_ЗДЕСЬ":
        print("Пожалуйста, вставьте ваш ключ от 'JavaScript API и HTTP Геокодер' в переменную GEOCODER_API_KEY в коде.")
        return

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
            print(f"Геокодинг нерезидента: {name[:30]}... ({addr[:40]})")
            res = geocode_address_yandex(addr, name)
            if res:
                lat, lng, found_addr = res
                coords[name] = {"lat": lat, "lng": lng, "address": found_addr}
                print(f"  ✅ Найдено: {found_addr}")
                updated = True
            else:
                print("  ❌ Не найдено. Оставляем центр Уральска.")
                coords[name] = {"lat": URALSK_CENTER[0], "lng": URALSK_CENTER[1], "address": addr}
                updated = True
            time.sleep(1) 
            
    if updated or not os.path.exists(coords_file):
        os.makedirs(os.path.dirname(coords_file), exist_ok=True)
        with open(coords_file, 'w', encoding='utf-8') as f:
            json.dump(coords, f, ensure_ascii=False, indent=2)
        print(f"\nСохранено {len(coords)} нерезидентов в {coords_file}")
    else:
        print("\nНовых нерезидентов нет. Координаты актуальны.")

if __name__ == "__main__":
    main()
