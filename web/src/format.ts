import type { Direction, Granularity } from './api';

const rubFull = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});
const rubWhole = new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB', maximumFractionDigits: 0 });
const compact = new Intl.NumberFormat('ru-RU', { notation: 'compact', maximumFractionDigits: 1 });
const intFmt = new Intl.NumberFormat('ru-RU');

const MONTHS_SHORT = ['янв', 'фев', 'мар', 'апр', 'мая', 'июн', 'июл', 'авг', 'сен', 'окт', 'ноя', 'дек'];
const MONTHS_FULL = [
  'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
  'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь',
];
const WEEKDAYS_SHORT = ['вс', 'пн', 'вт', 'ср', 'чт', 'пт', 'сб'];
const MONTHS_LOWER = [
  'январь', 'февраль', 'март', 'апрель', 'май', 'июнь',
  'июль', 'август', 'сентябрь', 'октябрь', 'ноябрь', 'декабрь',
];

/** "сентябрь 2026" from "2026-09", for phrases like "бюджет на ...". */
export function monthLabel(month: string): string {
  const [y, m] = month.split('-').map(Number);
  return `${MONTHS_LOWER[m - 1] ?? month} ${y}`;
}

/** Full amount with kopecks, e.g. "1 234,56 ₽". */
export function money(kopecks: number): string {
  return rubFull.format(kopecks / 100);
}

/** Amount rounded to rubles for tiles and bars. */
export function moneyWhole(kopecks: number): string {
  return rubWhole.format(kopecks / 100);
}

/** Short axis tick, e.g. "12,9 тыс. ₽". */
export function moneyCompact(kopecks: number): string {
  if (kopecks === 0) return '0';
  return `${compact.format(kopecks / 100)} ₽`;
}

export function signedMoney(kopecks: number, direction: Direction): string {
  return (direction === 'income' ? '+' : '−') + money(kopecks);
}

export function int(n: number): string {
  return intFmt.format(n);
}

/** Parses YYYY-MM-DD into a local Date at midnight. */
export function parseISODate(s: string): Date {
  const [y, m, d] = s.split('-').map(Number);
  return new Date(y, m - 1, d);
}

export function toISODate(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

export function addDays(iso: string, n: number): string {
  const d = parseISODate(iso);
  d.setDate(d.getDate() + n);
  return toISODate(d);
}

/** Inclusive number of calendar days between two ISO dates. */
export function daysBetween(from: string, to: string): number {
  const a = parseISODate(from);
  const b = parseISODate(to);
  return Math.round((b.getTime() - a.getTime()) / 86_400_000) + 1;
}

/** "12 сен" */
export function fmtDay(iso: string): string {
  const d = parseISODate(iso);
  return `${d.getDate()} ${MONTHS_SHORT[d.getMonth()]}`;
}

/** "12.09.2026" */
export function fmtDayFull(iso: string): string {
  const [y, m, d] = iso.split('-');
  return `${d}.${m}.${y}`;
}

/** "12.09.2026, сб" */
export function fmtDayWithWeekday(iso: string): string {
  const d = parseISODate(iso);
  return `${fmtDayFull(iso)}, ${WEEKDAYS_SHORT[d.getDay()]}`;
}

/** Renders an RFC 3339 timestamp as printed by the bank, without
 *  converting to the browser zone: "10.09.2026 17:41". */
export function fmtDateTime(rfc3339: string): string {
  if (!rfc3339) return '';
  const [date, rest] = rfc3339.split('T');
  const time = rest ? rest.slice(0, 5) : '';
  return `${fmtDayFull(date)} ${time}`.trim();
}

/** "17:41" from an RFC 3339 timestamp, without zone conversion. */
export function fmtTime(rfc3339: string): string {
  if (!rfc3339) return '';
  const rest = rfc3339.split('T')[1];
  return rest ? rest.slice(0, 5) : '';
}

/** Short x-axis label for a bucket start. */
export function bucketLabel(start: string, g: Granularity): string {
  const d = parseISODate(start);
  switch (g) {
    case 'month':
      return `${MONTHS_SHORT[d.getMonth()]} ${d.getFullYear()}`;
    case 'week':
    case 'day':
    default:
      return fmtDay(start);
  }
}

/** Long label for tooltips and the table view. */
export function bucketTitle(start: string, g: Granularity): string {
  const d = parseISODate(start);
  switch (g) {
    case 'month':
      return `${MONTHS_FULL[d.getMonth()]} ${d.getFullYear()}`;
    case 'week': {
      const end = addDays(start, 6);
      return `${fmtDayFull(start)} – ${fmtDayFull(end)}`;
    }
    case 'day':
    default:
      return fmtDayWithWeekday(start);
  }
}

/** Percentage change from prev to cur, or null when prev is zero. */
export function pctChange(cur: number, prev: number): number | null {
  if (prev <= 0) return null;
  return Math.round(((cur - prev) / prev) * 100);
}

export function plural(n: number, forms: [string, string, string]): string {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return forms[0];
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return forms[1];
  return forms[2];
}

const LEGAL_FORMS: Array<[RegExp, string]> = [
  [/^ОБЩЕСТВО С ОГРАНИЧЕННОЙ ОТВЕТСТВЕННОСТЬЮ\s*/i, 'ООО '],
  [/^ПУБЛИЧНОЕ АКЦИОНЕРНОЕ ОБЩЕСТВО\s*/i, 'ПАО '],
  [/^АКЦИОНЕРНОЕ ОБЩЕСТВО\s*/i, 'АО '],
  [/^ИНДИВИДУАЛЬНЫЙ ПРЕДПРИНИМАТЕЛЬ\s*/i, 'ИП '],
];

/** Shortens the legal form printed on receipts: "ОБЩЕСТВО С ... "ЯНДЕКС"" -> "ООО "ЯНДЕКС"". */
export function shortSeller(name: string): string {
  for (const [re, short] of LEGAL_FORMS) {
    if (re.test(name)) return short + name.replace(re, '').trim();
  }
  return name;
}

export const KIND_LABELS: Record<string, string> = {
  purchase: 'покупка',
  transfer_in: 'входящий перевод',
  transfer_out: 'исходящий перевод',
  refund: 'возврат',
  cashback: 'кэшбэк',
  other: 'другое',
};

export const FIELD_LABELS: Record<string, string> = {
  merchant: 'Продавец',
  description: 'Описание',
  mcc: 'MCC',
  kind: 'Тип операции',
  counterparty: 'Контрагент',
};

export const OP_LABELS: Record<string, string> = {
  contains: 'содержит',
  equals: 'равно',
  prefix: 'начинается с',
  regex: 'регулярное выражение',
};
