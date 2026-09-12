import { useCallback, useMemo } from 'react';
import { useSearchParams } from 'react-router-dom';

import type { DataRange, Granularity } from './api';
import { addDays, daysBetween, toISODate } from './format';

export interface Period {
  from: string;
  to: string;
}

export interface Preset {
  key: string;
  label: string;
  range: (today: string, data: DataRange | null) => Period;
}

function startOfMonth(iso: string): string {
  return `${iso.slice(0, 7)}-01`;
}

function startOfWeek(iso: string): string {
  const d = new Date(iso);
  const offset = (d.getUTCDay() + 6) % 7;
  return addDays(iso, -offset);
}

export const PRESETS: Preset[] = [
  { key: 'week', label: 'Эта неделя', range: (t) => ({ from: startOfWeek(t), to: t }) },
  { key: 'month', label: 'Этот месяц', range: (t) => ({ from: startOfMonth(t), to: t }) },
  {
    key: 'prev-month',
    label: 'Прошлый месяц',
    range: (t) => {
      const to = addDays(startOfMonth(t), -1);
      return { from: startOfMonth(to), to };
    },
  },
  { key: '30d', label: '30 дней', range: (t) => ({ from: addDays(t, -29), to: t }) },
  { key: '90d', label: '90 дней', range: (t) => ({ from: addDays(t, -89), to: t }) },
  { key: 'year', label: 'Этот год', range: (t) => ({ from: `${t.slice(0, 4)}-01-01`, to: t }) },
  {
    key: 'all',
    label: 'Всё время',
    range: (t, data) => (data && data.has_data ? { from: data.from, to: data.to } : { from: `${t.slice(0, 4)}-01-01`, to: t }),
  },
];

const DEFAULT_PRESET = 'month';

/** Picks a bucket size that keeps the chart readable for the span. */
export function autoGranularity(p: Period): Granularity {
  const n = daysBetween(p.from, p.to);
  if (n <= 62) return 'day';
  if (n <= 370) return 'week';
  return 'month';
}

export const GRANULARITY_LABELS: Record<Granularity, string> = {
  day: 'По дням',
  week: 'По неделям',
  month: 'По месяцам',
};

export interface PeriodState {
  preset: string;
  period: Period;
  granularity: Granularity;
  includeTransfers: boolean;
  setPreset: (key: string) => void;
  setCustom: (from: string, to: string) => void;
  setGranularity: (g: Granularity) => void;
  setIncludeTransfers: (on: boolean) => void;
}

/**
 * Period, granularity and the transfers toggle live in the URL so a refresh
 * or a shared link keeps the same slice, and every page reads one source.
 */
export function usePeriodState(data: DataRange | null): PeriodState {
  const [sp, setSp] = useSearchParams();
  const today = toISODate(new Date());

  const fromParam = sp.get('from');
  const toParam = sp.get('to');
  const presetParam = sp.get('preset');
  const preset = presetParam ?? (fromParam && toParam ? 'custom' : DEFAULT_PRESET);

  const period = useMemo<Period>(() => {
    if (preset === 'custom' && fromParam && toParam && fromParam <= toParam) {
      return { from: fromParam, to: toParam };
    }
    const p = PRESETS.find((x) => x.key === preset) ?? PRESETS.find((x) => x.key === DEFAULT_PRESET)!;
    return p.range(today, data);
  }, [preset, fromParam, toParam, today, data]);

  const gParam = sp.get('g') as Granularity | null;
  const granularity: Granularity = gParam === 'day' || gParam === 'week' || gParam === 'month' ? gParam : autoGranularity(period);
  const includeTransfers = sp.get('transfers') === '1';

  const update = useCallback(
    (patch: Record<string, string | null>) => {
      setSp(
        (prev) => {
          const next = new URLSearchParams(prev);
          for (const [k, v] of Object.entries(patch)) {
            if (v === null) next.delete(k);
            else next.set(k, v);
          }
          return next;
        },
        { replace: true },
      );
    },
    [setSp],
  );

  return {
    preset,
    period,
    granularity,
    includeTransfers,
    setPreset: (key) => update({ preset: key, from: null, to: null, g: null }),
    setCustom: (from, to) => {
      if (!from || !to) return;
      update({ preset: 'custom', from, to, g: null });
    },
    setGranularity: (g) => update({ g }),
    setIncludeTransfers: (on) => update({ transfers: on ? '1' : null }),
  };
}
