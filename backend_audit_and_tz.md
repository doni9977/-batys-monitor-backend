# BatysMonitor Backend — Аудит ошибок, план улучшений и ТЗ для ИИ-агента

---

## ЧАСТЬ 1. НАЙДЕННЫЕ ОШИБКИ И ПРОБЛЕМЫ

---

### 🔴 КРИТИЧЕСКИЕ ОШИБКИ (ломают корректность данных)

---

#### БАГ #1 — Дублирование данных при повторной загрузке
**Файл:** `internal/handlers/upload.go`

**Проблема:** При каждом вызове `POST /api/upload` новые данные добавляются в таблицу `service_records` без очистки старых. После двух загрузок одного файла в БД 143,934 строк вместо 71,967. Все алгоритмы рисков дают удвоенные результаты.

**Доказательство из тестирования:**
- Job 1 (после первой загрузки): A1 = 64, A2 = 12
- Job 2 (после второй загрузки тех же данных): A1 = 128, A2 = 24 — ровно в 2 раза больше

**Текущий код:**
```go
result := database.DB.CreateInBatches(&records, 1000)
```

**Как исправить:**
```go
// Очищаем старые данные ДО вставки новых
database.DB.Exec("TRUNCATE TABLE service_records RESTART IDENTITY;")
result := database.DB.CreateInBatches(&records, 1000)
```

---

#### БАГ #2 — A4 реализован по неправильной логике
**Файл:** `internal/services/risk_engine.go`, функция `collectA4()`

**Проблема:** Go-бэкенд ищет только **кросс-клиничные** дубликаты (одна услуга одному пациенту в разных клиниках). По методологии ТЗ A4 должен ловить **любое превышение дневного лимита** (`max_per_day` из классификатора) у одного врача для одного пациента. В тестовых данных кросс-клиничных дублей нет → A4 всегда возвращает 0.

**Текущий SQL (неверный):**
```sql
GROUP BY patient_iin, service_code, service_date::date
HAVING COUNT(*) > 1
   AND COUNT(DISTINCT clinic_name) > 1  -- ← это ищет только кросс-клиничный фрод
```

**Правильный SQL:**
```sql
SELECT
    sr.patient_iin,
    sr.doctor_name,
    sr.service_code,
    MIN(sr.service_name) AS service_name,
    sr.service_date::date AS risk_date,
    COUNT(*) AS total_count,
    sc.max_per_day AS allowed_per_day,
    COALESCE(SUM(sr.amount), 0) AS total_amount
FROM service_records AS sr
JOIN service_classifiers AS sc ON sc.code = sr.service_code
WHERE BTRIM(COALESCE(sr.patient_iin, '')) <> ''
  AND sc.max_per_day > 0
GROUP BY sr.patient_iin, sr.doctor_name, sr.service_code, sr.service_date::date, sc.max_per_day
HAVING COUNT(*) > sc.max_per_day
```

---

#### БАГ #3 — Неправильная колонка для парсинга ИИН
**Файл:** `internal/parser/excel_parser.go`

**Проблема:** `findColIndex("иин", "инн")` — ищет первое совпадение. В файле Excel есть ДВА столбца со словом "иин": `ИИНы` (пациент, индекс 15) и `Идентификатор пациента` (индекс 7). Парсер может подцепить неверный столбец. Также нет явного маппинга на `Работник поставщика` (врач) — сейчас он ищет по "врач", но в Excel этот столбец называется "Врачи".

**Как исправить:** Сделать маппинг заголовков жёстким, с фолбэком:
```go
iinIdx := findColIndex("иины", "иин пациента", "иин")
doctorIdx := findColIndex("врачи", "врач", "специалист", "работник")
clinicIdx := findColIndex("поставщик", "наименование мо", "организация")
```

---

### 🟡 ЛОГИЧЕСКИЕ ОШИБКИ (алгоритм работает, но неточно)

---

#### БАГ #4 — A3: неправильный порог и единица измерения
**Файл:** `internal/services/risk_engine.go`, функция `collectA3()`

**Проблема:** Порог `COUNT(*) > 200` — это **200 услуг в день**. По методологии ТЗ порог — **80 услуг в час** (документ раздел 4.1, A3: "200 услуг/день или 80 услуг/час — оба триггера"). Сейчас ловит только 3 случая вместо 40.

**Правильный SQL:**
```sql
-- Группировка по врачу + дата + ЧАС
SELECT
    clinic_name,
    doctor_name,
    DATE_TRUNC('hour', service_date) AS service_hour,
    COUNT(*) AS service_count
FROM service_records
WHERE BTRIM(COALESCE(doctor_name, '')) <> ''
GROUP BY clinic_name, doctor_name, DATE_TRUNC('hour', service_date)
HAVING COUNT(*) > 80   -- 80 услуг в час

UNION ALL

-- Также группировка по дню
SELECT
    clinic_name,
    doctor_name,
    DATE_TRUNC('day', service_date) AS service_hour,
    COUNT(*) AS service_count
FROM service_records
WHERE BTRIM(COALESCE(doctor_name, '')) <> ''
GROUP BY clinic_name, doctor_name, DATE_TRUNC('day', service_date)
HAVING COUNT(*) > 200   -- 200 услуг в день
```

---

#### БАГ #5 — A8: нет допуска на погрешность тарифа
**Файл:** `internal/services/risk_engine.go`, функция `collectA8()`

**Проблема:** Текущее условие `amount > tariff * qty` флагирует любое превышение даже на 1 тенге. Классификатор — демо-данные с округлёнными тарифами. Результат: 29,982 ложных срабатываний вместо реальных ~191 значимых случаев.

**Правильное условие (с допуском 20%):**
```sql
WHERE sc.tariff > 0
  AND sr.amount > sc.tariff * GREATEST(sr.quantity, 1) * 1.20
```

---

#### БАГ #6 — A10: исключает слишком много данных фильтром `00:00:00`
**Файл:** `internal/services/risk_engine.go`, функция `collectA10()`

**Проблема:** Фильтр `service_date::time <> TIME '00:00:00'` правильный (нельзя сравнивать интервалы без точного времени), но Go-парсер использует `Период услуги` (с полным временем), поэтому записей с временем 00:00:00 практически нет. Проблема в другом: `PARTITION BY sr.doctor_name` не учитывает дату — врач может стоять последним в цепочке одного дня и первым следующего, интервал будет огромным, но система всё равно его проверит.

**Правильное окно:**
```sql
WINDOW physician_services AS (
    PARTITION BY sr.doctor_name, sr.service_date::date  -- добавить дату!
    ORDER BY sr.service_date, sr.id
)
```

---

### 🟠 АРХИТЕКТУРНЫЕ ПРОБЛЕМЫ

---

#### БАГ #7 — DSN (пароль БД) захардкожен в коде
**Файл:** `internal/database/db.go`

```go
// ПЛОХО — пароль в коде
dsn := "host=localhost user=batys_user password=batys_password dbname=batys_monitor port=5440"
```

Нужно читать из `.env` файла через уже установленную библиотеку `joho/godotenv`.

---

#### БАГ #8 — Нет CORS — фронтенд не сможет подключиться
**Файл:** `cmd/main.go`

Next.js фронтенд не сможет делать запросы к API без заголовков CORS. Middleware не установлен.

---

#### БАГ #9 — Временный файл классификатора не удаляется
**Файл:** `internal/handlers/classifier_upload.go`

```go
tempPath := filepath.Join("./", file.Filename)
c.SaveFile(file, tempPath)
// ... нет defer os.Remove(tempPath) !
```

---

#### БАГ #10 — Нет пагинации в API рисков
**Файл:** `internal/handlers/risk_query.go`

A10 возвращает 23,399 записей одним JSON. При большом объёме данных это:
- Таймаут HTTP-запроса
- OOM на сервере
- Зависание браузера

---

---

## ЧАСТЬ 2. ПЛАН УЛУЧШЕНИЙ ДО УРОВНЯ ГОТОВОГО ПРОДУКТА

---

### 🔒 Безопасность и конфигурация
- [ ] Все секреты (DSN, порты) вынести в `.env` файл
- [ ] Добавить CORS middleware с поддержкой конкретных origins
- [ ] Добавить базовую JWT-авторизацию для всех `/api/*` endpoints
- [ ] Rate limiting на upload endpoint (файл 27 МБ = нагрузка)

### 📦 Качество данных
- [ ] TRUNCATE `service_records` перед каждой новой загрузкой (БАГ #1)
- [ ] Версионирование загрузок: хранить `upload_id` и видеть историю
- [ ] Валидация файла: проверять наличие обязательных колонок до парсинга
- [ ] Статус загрузки в реальном времени (WebSocket или SSE прогресс-бар)

### 🧠 Алгоритмы рисков
- [ ] Исправить A4 (БАГ #2) — дневной лимит из классификатора
- [ ] Исправить A3 (БАГ #4) — порог 80/час и 200/день
- [ ] Исправить A8 (БАГ #5) — допуск 20%
- [ ] Исправить A10 (БАГ #6) — PARTITION BY включает дату
- [ ] Добавить расчёт `risk_score` (числовой вес) для каждого индикатора
- [ ] Добавить агрегацию по МО: суммарный `risk_score` по клинике для карты

### 🌐 API
- [ ] Пагинация: `?page=1&limit=100` для всех GET /api/risks/*
- [ ] Фильтрация: `?doctor=...&date_from=...&date_to=...&clinic=...`
- [ ] Новый endpoint: `GET /api/summary` — сводка по всем алгоритмам одним запросом
- [ ] Новый endpoint: `GET /api/clinics/risks` — список клиник с суммарным риск-баллом для карты
- [ ] Новый endpoint: `GET /api/export/xlsx` — экспорт найденных рисков в Excel
- [ ] Swagger/OpenAPI документация

### 🏗️ Инфраструктура
- [ ] Добавить `docker-compose.override.yml` для локальной разработки
- [ ] Healthcheck для PostgreSQL в docker-compose
- [ ] Индексы в PostgreSQL на часто используемые поля
- [ ] Graceful shutdown сервера

---

---

## ЧАСТЬ 3. ПОЛНОЕ ТЗ ДЛЯ ИИ-АГЕНТА

---

# ТЕХНИЧЕСКОЕ ЗАДАНИЕ
## Доработка бэкенда BatysMonitor до уровня production-ready

**Репозиторий:** `/Users/danialmarat/Documents/проект/batys-monitor-backend`
**Язык:** Go (Golang), фреймворк Fiber v2, ORM GORM, БД PostgreSQL
**Цель:** Исправить все найденные баги, улучшить алгоритмы, добавить новые API-эндпоинты

---

### ЗАДАЧА 1. Конфигурация через `.env` файл

**Что сделать:**
1. Создать файл `.env` в корне проекта:
```
DB_HOST=localhost
DB_PORT=5440
DB_USER=batys_user
DB_PASSWORD=batys_password
DB_NAME=batys_monitor
DB_TIMEZONE=Asia/Oral
SERVER_PORT=3000
CORS_ORIGINS=http://localhost:3001,http://localhost:3002
```
2. В `internal/database/db.go` добавить `godotenv.Load()` и читать DSN из `os.Getenv()`:
```go
import "github.com/joho/godotenv"

func ConnectDb() {
    godotenv.Load() // загружаем .env
    dsn := fmt.Sprintf(
        "host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=%s",
        os.Getenv("DB_HOST"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"),
        os.Getenv("DB_NAME"), os.Getenv("DB_PORT"), os.Getenv("DB_TIMEZONE"),
    )
    // ...
}
```
3. В `cmd/main.go` порт сервера тоже читать из ENV.
4. Добавить `.env` в `.gitignore`, создать `.env.example` с пустыми значениями.

---

### ЗАДАЧА 2. CORS middleware

**Что сделать:**
В `cmd/main.go` добавить CORS middleware от Fiber до регистрации роутов:
```go
import "github.com/gofiber/fiber/v2/middleware/cors"

app.Use(cors.New(cors.Config{
    AllowOrigins: os.Getenv("CORS_ORIGINS"),
    AllowHeaders: "Origin, Content-Type, Accept, Authorization",
    AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
}))
```
Установить пакет: `go get github.com/gofiber/fiber/v2/middleware/cors`

---

### ЗАДАЧА 3. Исправить дублирование данных (БАГ #1)

**Файл:** `internal/handlers/upload.go`

**Что сделать:** Добавить TRUNCATE перед вставкой:
```go
// Очищаем таблицу перед новой загрузкой
if err := database.DB.Exec("TRUNCATE TABLE service_records RESTART IDENTITY;").Error; err != nil {
    return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
        "error": "Не удалось очистить старые данные: " + err.Error(),
    })
}
log.Println("Таблица service_records очищена. Начинаем загрузку новых данных...")
result := database.DB.CreateInBatches(&records, 1000)
```

---

### ЗАДАЧА 4. Исправить A4 — правильная логика дублирования (БАГ #2)

**Файл:** `internal/services/risk_engine.go`, функция `collectA4()`

**Что сделать:** Полностью переписать SQL-запрос:
```sql
SELECT
    MIN(sr.clinic_name) AS clinic_name,
    sr.doctor_name,
    sr.patient_iin,
    sr.service_code,
    MIN(sr.service_name) AS service_name,
    sr.service_date::date AS risk_date,
    COUNT(*) AS total_count,
    MAX(sc.max_per_day) AS allowed_per_day,
    COALESCE(SUM(sr.amount), 0) AS total_amount
FROM service_records AS sr
JOIN service_classifiers AS sc ON sc.code = sr.service_code
WHERE BTRIM(COALESCE(sr.patient_iin, '')) <> ''
  AND BTRIM(COALESCE(sr.doctor_name, '')) <> ''
  AND sc.max_per_day > 0
GROUP BY sr.patient_iin, sr.doctor_name, sr.service_code, sr.service_date::date
HAVING COUNT(*) > MAX(sc.max_per_day)
ORDER BY total_count DESC
```

Обновить структуру `row` соответственно:
```go
type row struct {
    ClinicName    string
    DoctorName    string
    PatientIIN    string
    ServiceCode   string
    ServiceName   string
    RiskDate      time.Time
    TotalCount    int
    AllowedPerDay int
    TotalAmount   float64
}
```

---

### ЗАДАЧА 5. Исправить A3 — порог 80/час (БАГ #4)

**Файл:** `internal/services/risk_engine.go`, функция `collectA3()`

**Что сделать:** Изменить SQL для группировки по часам:
```sql
SELECT
    clinic_name,
    doctor_name,
    DATE_TRUNC('hour', service_date)::timestamp AS service_hour,
    COUNT(*) AS service_count
FROM service_records
WHERE BTRIM(COALESCE(clinic_name, '')) <> ''
  AND BTRIM(COALESCE(doctor_name, '')) <> ''
  AND service_date IS NOT NULL
GROUP BY clinic_name, doctor_name, DATE_TRUNC('hour', service_date)
HAVING COUNT(*) > 80
```

Обновить структуру `row`:
```go
type row struct {
    ClinicName   string
    DoctorName   string
    ServiceHour  time.Time
    ServiceCount int
}
```

В деталях сохранять: `threshold: 80, unit: "per_hour"`.

---

### ЗАДАЧА 6. Исправить A8 — добавить 20% допуск (БАГ #5)

**Файл:** `internal/services/risk_engine.go`, функция `collectA8()`

**Что сделать:** Изменить WHERE условие:
```sql
WHERE sc.tariff > 0
  AND sr.amount > sc.tariff * GREATEST(sr.quantity, 1) * 1.20
```

---

### ЗАДАЧА 7. Исправить A10 — PARTITION включает дату (БАГ #6)

**Файл:** `internal/services/risk_engine.go`, функция `collectA10()`

**Что сделать:** Изменить WINDOW:
```sql
-- БЫЛО:
WINDOW physician_services AS (PARTITION BY sr.doctor_name ORDER BY sr.service_date, sr.id)

-- СТАЛО:
WINDOW physician_services AS (
    PARTITION BY sr.doctor_name, sr.service_date::date
    ORDER BY sr.service_date, sr.id
)
```

---

### ЗАДАЧА 8. Исправить утечку файла классификатора (БАГ #9)

**Файл:** `internal/handlers/classifier_upload.go`

**Что сделать:** Добавить удаление временного файла:
```go
tempPath := filepath.Join("./tmp/", file.Filename)
os.MkdirAll("./tmp", os.ModePerm)
if err := c.SaveFile(file, tempPath); err != nil { ... }
defer os.Remove(tempPath) // добавить эту строку
```

---

### ЗАДАЧА 9. Добавить пагинацию в API рисков

**Файл:** `internal/handlers/risk_query.go`

**Что сделать:** Обновить функцию `loadRisksByIndicator()`:
```go
func loadRisksByIndicator(c *fiber.Ctx, indicator string) ([]models.DetectedRisk, int64, error) {
    // ... существующая логика фильтрации по job_id ...

    // Пагинация
    page, _ := strconv.Atoi(c.Query("page", "1"))
    limit, _ := strconv.Atoi(c.Query("limit", "100"))
    if limit > 1000 { limit = 1000 }
    if page < 1 { page = 1 }
    offset := (page - 1) * limit

    var total int64
    query.Count(&total)

    var risks []models.DetectedRisk
    query.Offset(offset).Limit(limit).Order("risk_date DESC, id DESC").Find(&risks)

    return risks, total, nil
}
```

Обновить все handler-функции чтобы возвращали `total`, `page`, `limit` в ответе:
```json
{
  "indicator": "A10",
  "total_found": 23399,
  "page": 1,
  "limit": 100,
  "total_pages": 234,
  "risks": [...]
}
```

---

### ЗАДАЧА 10. Новый endpoint — сводка `/api/summary`

**Создать файл:** `internal/handlers/summary.go`

**Что сделать:**
```go
func GetRiskSummary(c *fiber.Ctx) error {
    // Один SQL-запрос для сводки по всем индикаторам
    // GROUP BY indicator
    // Возвращает: indicator, count, total_amount для каждого
}
```

SQL:
```sql
SELECT
    indicator,
    COUNT(*) AS risk_count,
    COALESCE(SUM(amount), 0) AS total_amount
FROM detected_risks
WHERE job_id = (
    SELECT id FROM risk_jobs
    WHERE status = 'done'
    ORDER BY created_at DESC LIMIT 1
)
GROUP BY indicator
ORDER BY indicator
```

Зарегистрировать в `cmd/main.go`:
```go
app.Get("/api/summary", handlers.GetRiskSummary)
```

---

### ЗАДАЧА 11. Новый endpoint — агрегация по клиникам `/api/clinics/risks`

**Создать файл:** `internal/handlers/clinics.go`

**Назначение:** Этот endpoint нужен фронтенду для отображения клиник на карте Mapbox с цветовой индикацией (светофор).

**Что вернуть:**
```json
[
  {
    "clinic_name": "Городская поликлиника №5",
    "risk_score": 1500,
    "risk_level": "red",
    "a1_count": 5, "a2_count": 0, "a3_count": 12,
    "a4_count": 200, "a7_count": 30, "a8_count": 0, "a10_count": 1253,
    "total_amount": 15820383.00
  }
]
```

SQL:
```sql
SELECT
    clinic_name,
    COUNT(*) FILTER (WHERE indicator = 'A1') AS a1_count,
    COUNT(*) FILTER (WHERE indicator = 'A2') AS a2_count,
    COUNT(*) FILTER (WHERE indicator = 'A3') AS a3_count,
    COUNT(*) FILTER (WHERE indicator = 'A4') AS a4_count,
    COUNT(*) FILTER (WHERE indicator = 'A7') AS a7_count,
    COUNT(*) FILTER (WHERE indicator = 'A8') AS a8_count,
    COUNT(*) FILTER (WHERE indicator = 'A10') AS a10_count,
    COUNT(*) AS total_risks,
    COALESCE(SUM(amount), 0) AS total_amount,
    -- Риск-скор: каждый индикатор имеет свой вес
    (COUNT(*) FILTER (WHERE indicator = 'A5') * 100 +  -- умершие = 100 баллов
     COUNT(*) FILTER (WHERE indicator = 'A3') * 50  +  -- нагрузка = 50 баллов
     COUNT(*) FILTER (WHERE indicator = 'A4') * 30  +  -- дублирование = 30 баллов
     COUNT(*) FILTER (WHERE indicator = 'A10') * 10 +  -- интервал = 10 баллов
     COUNT(*) FILTER (WHERE indicator = 'A8') * 20  +  -- upcoding = 20 баллов
     COUNT(*) FILTER (WHERE indicator = 'A7') * 15  +  -- физлимит = 15 баллов
     COUNT(*) FILTER (WHERE indicator = 'A1') * 5   +  -- возраст = 5 баллов
     COUNT(*) FILTER (WHERE indicator = 'A2') * 5      -- пол = 5 баллов
    ) AS risk_score
FROM detected_risks
WHERE job_id = (SELECT id FROM risk_jobs WHERE status='done' ORDER BY created_at DESC LIMIT 1)
  AND clinic_name IS NOT NULL AND clinic_name <> ''
GROUP BY clinic_name
ORDER BY risk_score DESC
```

Уровень риска вычислять в Go:
```go
if riskScore > 1000 { level = "red" }
else if riskScore > 200 { level = "yellow" }
else { level = "green" }
```

---

### ЗАДАЧА 12. Endpoint для экспорта в Excel `/api/export/xlsx`

**Создать файл:** `internal/handlers/export.go`

**Что сделать:** Использовать `excelize` для генерации Excel-отчёта с найденными рисками.
- Отдельный лист для каждого индикатора (A1, A2, A3...)
- Первый лист — сводная таблица (Summary)
- Content-Type: `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`
- Content-Disposition: `attachment; filename="BatysMonitor_Риски_<date>.xlsx"`

---

### ЗАДАЧА 13. Индексы в PostgreSQL для производительности

**Что сделать:** Добавить в `internal/database/db.go` после AutoMigrate:
```go
// Индексы для ускорения SQL-запросов алгоритмов
db.Exec("CREATE INDEX IF NOT EXISTS idx_sr_doctor_date ON service_records(doctor_name, service_date);")
db.Exec("CREATE INDEX IF NOT EXISTS idx_sr_patient_code_date ON service_records(patient_iin, service_code, service_date);")
db.Exec("CREATE INDEX IF NOT EXISTS idx_sr_code ON service_records(service_code);")
db.Exec("CREATE INDEX IF NOT EXISTS idx_dr_indicator_job ON detected_risks(indicator, job_id);")
db.Exec("CREATE INDEX IF NOT EXISTS idx_dr_clinic ON detected_risks(clinic_name);")
```

---

### ЗАДАЧА 14. Улучшить docker-compose.yml

**Файл:** `docker-compose.yml`

**Что добавить:**
```yaml
services:
  postgres:
    image: postgis/postgis:15-3.3
    platform: linux/amd64
    environment:
      POSTGRES_USER: ${DB_USER:-batys_user}
      POSTGRES_PASSWORD: ${DB_PASSWORD:-batys_password}
      POSTGRES_DB: ${DB_NAME:-batys_monitor}
    ports:
      - "${DB_PORT:-5440}:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U batys_user -d batys_monitor"]
      interval: 10s
      timeout: 5s
      retries: 5
volumes:
  pgdata:
```

---

### ЗАДАЧА 15. README.md с инструкцией запуска

**Создать файл:** `README.md`

Содержание:
1. Описание проекта (BatysMonitor — система выявления медицинских рисков)
2. Требования (Go 1.21+, Docker, Docker Compose)
3. Быстрый старт:
```bash
git clone ...
cp .env.example .env
docker compose up -d
go run ./cmd/main.go
```
4. Описание всех API-эндпоинтов с примерами curl
5. Описание алгоритмов (A1-A10)
6. Описание формата входного Excel-файла

---

## ПОРЯДОК ВЫПОЛНЕНИЯ ЗАДАЧ ДЛЯ ИИ-АГЕНТА

Выполнять в следующем порядке (от критичного к улучшениям):

1. **ЗАДАЧА 1** — `.env` конфигурация
2. **ЗАДАЧА 2** — CORS middleware  
3. **ЗАДАЧА 3** — TRUNCATE при загрузке (критичный баг)
4. **ЗАДАЧА 4** — Исправить A4
5. **ЗАДАЧА 5** — Исправить A3
6. **ЗАДАЧА 6** — Исправить A8
7. **ЗАДАЧА 7** — Исправить A10
8. **ЗАДАЧА 8** — Исправить утечку файла
9. **ЗАДАЧА 13** — Индексы PostgreSQL
10. **ЗАДАЧА 9** — Пагинация API
11. **ЗАДАЧА 10** — Endpoint /api/summary
12. **ЗАДАЧА 11** — Endpoint /api/clinics/risks
13. **ЗАДАЧА 12** — Экспорт в Excel
14. **ЗАДАЧА 14** — docker-compose healthcheck
15. **ЗАДАЧА 15** — README.md

После каждой задачи: `go build ./...` для проверки компиляции.
После задач 1-8: перезапустить сервер, загрузить тестовые файлы и проверить что результаты не дублируются.

---

## ОЖИДАЕМЫЕ РЕЗУЛЬТАТЫ ПОСЛЕ ИСПРАВЛЕНИЙ

| Алгоритм | До (баги) | После (исправленный) |
|---|---|---|
| A1 | 128 (дубли) | ~64 |
| A2 | 24 (дубли) | ~12 |
| A3 | 3 (неверный порог) | ~40 |
| A4 | 0 (неверная логика) | ~1,777 |
| A7 | 1,395 (дубли) | ~335 |
| A8 | 29,982 (нет допуска) | ~191 |
| A10 | 23,399 | ~23,000 (незначительно изменится) |
| **Итого** | **55,030 (с дублями)** | **~61,000** |
