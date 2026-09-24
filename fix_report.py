import re

with open("scripts/generate_report.py", "r", encoding="utf-8") as f:
    content = f.read()

# 1. Fix NR5 description
old_nr5 = 'Установлено, что нижеперечисленные ТОО под руководством нерезидентов осуществляют вывод валютных ценностей за пределы Республики Казахстан.\\n\\nТовариществами заключены международные контракты на поставку товаров. Компании осуществили перевод денежных средств в адрес нерезидентов на крупные суммы.\\n\\nПри этом, по данным таможенных систем, фактическая поставка товара на территорию РК не осуществлена, возврат денежных средств не произведен. Вышеуказанные факты указывают на признаки уголовного правонарушения, предусмотренного ст. 235-1 УК РК (Незаконный вывод валютных ценностей).'
new_nr5 = 'Изучением финансово-хозяйственной деятельности компаний, зарегистрированных на нерезидентов, установлены признаки фиктивности и финансовой неактивности.\\n\\nПо данным банков второго уровня (БВУ), у нижеперечисленных юридических лиц отсутствуют открытые банковские счета, либо на имеющихся счетах наблюдается нулевой (минимальный) остаток средств при отсутствии движения.\\n\\nОтсутствие банковских счетов или операций по ним свидетельствует о невозможности ведения реальной предпринимательской деятельности и указывает на использование данных компаний в качестве номинальных структур (компаний-однодневок).'
content = content.replace(old_nr5, new_nr5)

# 2. Append get_ai_decision function and the footer loop
ai_decision_func = """
def get_ai_decision(indicator, d):
    try:
        if indicator in ["A1", "A2"]:
            return f"{d.get('reason', 'Нарушение')}: Пациенту {d.get('patient_age', '—')} лет, Пол: {d.get('patient_gender', '—')}"
        elif indicator == "A3":
            return f"Врач оказал {d.get('service_count', '')} услуг за час (норма {d.get('threshold', '')}) и {d.get('daily_count', '')} услуг за день (норма 200)"
        elif indicator == "A4":
            return f"Услуга оказана {d.get('total_count', '')} раз за день (ограничение: {d.get('allowed_per_day', '')} в день)"
        elif indicator == "A7":
            return f"За год услуга оказана {d.get('total_quantity', '')} раз (годовой лимит: {d.get('allowed_per_year', '')})"
        elif indicator == "A8":
            return f"Завышение стоимости: сумма к оплате {d.get('actual_amount', '')} ₸ (макс. тариф с учетом количества: {d.get('allowed_amount', '')} ₸). Разница: {d.get('excess_amount', '')} ₸"
        elif indicator == "A10":
            return f"Интервал между услугами составил {d.get('actual_interval_minutes', '')} мин (норматив {d.get('required_interval_minutes', '')} мин). Предыдущая услуга: {d.get('previous_service_name', '—')}"
        elif indicator == "S1":
            return f"Услуга '{d.get('service_name', '—')}' оказана в поликлинике {d.get('service_date', '')}, когда пациент находился в стационаре ({d.get('admission_date', '')} - {d.get('discharge_date', '')}). Физически невозможно."
        elif indicator == "S2":
            return f"Повторная госпитализация через {d.get('gap_days', '')} дн. с тем же диагнозом ({d.get('icd10_code', '')}). Предыдущая выписка: {d.get('prev_discharge', '')}, новое поступление: {d.get('new_admission_date', '')}. Признак дробления случая."
        elif indicator == "S3":
            return f"Круглосуточный стационар при пребывании {d.get('bed_days', '')} койко-дн. — дорогой тариф не соответствует сроку. Диагноз: {d.get('diagnosis', '—')} ({d.get('icd10_code', '')})."
        elif indicator == "S4":
            return f"{d.get('reason', '')}. Отделение: {d.get('department', '')}. Экстренных {d.get('emergency_patients', '')} из {d.get('total_patients', '')} ({d.get('emergency_percent', '')}%)."
        elif indicator == "S5":
            return f"Услуга '{d.get('service_name', '—')}' оказана в поликлинике {d.get('service_date', '')}, после зафиксированной даты смерти пациента ({d.get('death_date', '')})."
        elif indicator == "NR1":
            return f"Дата въезда: {d.get('entry_date')}, выезда: {d.get('exit_date')}. Регистрация ТОО ({d.get('reg_date')}) произведена в отсутствие нерезидента в стране."
        elif indicator == "NR2":
            return f"Массовый заезд. Транспорт: {d.get('vehicle_plate')}. КПП: {d.get('crossing_point')}."
        elif indicator == "NR3":
            return f"Нотариус: {d.get('notary')} / Переводчик: {d.get('translator')}. Участвовал в регистрации {d.get('companies_count')} рисковых компаний."
        elif indicator == "NR4":
            return f"Налоги: {d.get('taxes_paid')} ₸, Сотрудников: {d.get('employees_count')}. Финансовая пустышка."
        elif indicator == "NR5":
            return f"Статус счета: {d.get('account_status')}, Баланс: {d.get('balance')} ₸. {d.get('reason', 'Нет активности')}."
    except:
        pass
    return str(d)

def generate_algorithm_report"""

if "def get_ai_decision" not in content:
    content = content.replace("def generate_algorithm_report", ai_decision_func)

footer_code = """
    doc.add_page_break()
    h2 = doc.add_heading('Сводка по рискам и пояснения алгоритма (AI Decision)', level=2)
    
    for i, r in enumerate(risks):
        details = r.get('details', {})
        p = doc.add_paragraph()
        run = p.add_run(f"{i+1}. ")
        run.bold = True
        
        clin_label = 'Компания' if indicator.startswith('NR') else 'Клиника'
        doc_label = 'Директор' if indicator.startswith('NR') else 'Врач'
        pat_label = 'БИН' if indicator.startswith('NR') else 'Пациент'
        
        if r.get('clinic_name') and r.get('clinic_name') != '—':
            p.add_run(f"{clin_label}: ").bold = True
            p.add_run(f"{r.get('clinic_name')}\\n")
            
        if r.get('doctor_name') and r.get('doctor_name') != '—':
            p.add_run(f"   {doc_label}: ").bold = True
            p.add_run(f"{r.get('doctor_name')}\\n")
        
        pat_val = r.get('patient_iin')
        if pat_val and pat_val != '—' and pat_val != '0':
            p.add_run(f"   {pat_label}: ").bold = True
            p.add_run(f"{pat_val}\\n")
            
        date_str = r.get('risk_date', '')
        if date_str:
            p.add_run(f"   Дата фиксации: ").bold = True
            p.add_run(f"{date_str[:10]}\\n")
        
        ai_decision = get_ai_decision(indicator, details)
        p.add_run(f"   Детали ИИ: ").bold = True
        font = p.add_run(f"{ai_decision}\\n").font
        font.color.rgb = RGBColor(18, 97, 160)
        
        if r.get('amount', 0) > 0:
            p.add_run(f"   Сумма ущерба/капитала: ").bold = True
            run = p.add_run(f"{r.get('amount', 0):,.2f} ₸\\n")
            run.bold = True
            run.font.color.rgb = RGBColor(194, 24, 91)

    doc.save(output_path)
"""

if "Сводка по рискам и пояснения алгоритма" not in content:
    # replace the LAST doc.save(output_path) with footer_code
    parts = content.rsplit("    doc.save(output_path)", 1)
    if len(parts) == 2:
        content = parts[0] + footer_code + parts[1]

with open("scripts/generate_report.py", "w", encoding="utf-8") as f:
    f.write(content)

