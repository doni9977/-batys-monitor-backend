#!/usr/bin/env python3
import json, urllib.request, urllib.parse, time, os, subprocess

API_KEY = "0663c1de-1d43-41ce-883b-c71608710dc2"
URALSK_CENTER = (51.2333, 51.3667)

def geocode_yandex(query):
    # Очищаем название для лучшего поиска (Яндекс иногда плохо ищет слишком длинные официальные названия)
    search_query = query.replace('ГКП на праве хозяйственного ведения', '').replace('управления здравоохранения акимата Западно-Казахстанской области', '')
    search_query = search_query.strip() + ", Уральск, Казахстан"
    
    params = urllib.parse.urlencode({
        "text": search_query,
        "type": "biz",
        "lang": "ru_RU",
        "results": 1,
        "apikey": API_KEY
    })
    url = f"https://search-maps.yandex.ru/v1/?{params}"
    req = urllib.request.Request(url, headers={"User-Agent": "BatysMonitor/1.0"})
    
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            data = json.loads(resp.read())
            features = data.get("features", [])
            if features:
                # В GeoJSON возвращается [долгота, широта]
                lon, lat = features[0]["geometry"]["coordinates"]
                address = features[0]["properties"]["CompanyMetaData"].get("address", "")
                name = features[0]["properties"]["CompanyMetaData"].get("name", "")
                full_address = f"{name}, {address}"
                return float(lat), float(lon), full_address
    except urllib.error.HTTPError as e:
        if e.code == 403:
            print("  [ОШИБКА] Ключ API Яндекса недействителен или еще не активирован (обычно активация занимает 5-15 минут).")
        else:
            print(f"  [ОШИБКА] Yandex API HTTP Error {e.code}")
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
        if clinic not in coords or coords[clinic].get("lat") == URALSK_CENTER[0]:
            print(f"Поиск в Яндексе: {clinic}...")
            res = geocode_yandex(clinic)
            if res:
                lat, lng, address = res
                coords[clinic] = {"lat": lat, "lng": lng, "address": address}
                print(f"  ✅ Найдено: {address}")
                updated = True
            else:
                print("  ❌ Не найдено или ошибка. Оставляем центр Уральска.")
                # Если не нашли, оставляем заглушку
                coords[clinic] = {"lat": URALSK_CENTER[0], "lng": URALSK_CENTER[1], "address": f"г. Уральск ({clinic})"}
                updated = True
            time.sleep(1)
            
    if updated or not os.path.exists(coords_file):
        os.makedirs(os.path.dirname(coords_file), exist_ok=True)
        with open(coords_file, 'w', encoding='utf-8') as f:
            json.dump(coords, f, ensure_ascii=False, indent=2)
        print(f"\nСохранено {len(coords)} клиник в {coords_file}")
    else:
        print("\nНовых клиник нет. Координаты актуальны.")

if __name__ == "__main__":
    main()
