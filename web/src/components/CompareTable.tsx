import { useState } from 'react';

import type { Comparison } from '../api';
import { fmtDayFull, money, pctChange } from '../format';

interface Props {
  data: Comparison;
  loading?: boolean;
}

interface Row {
  key: string;
  name: string;
  color?: string;
  current: number;
  previous: number;
}

type Tab = 'categories' | 'merchants';

function DeltaRow({ name, color, current, previous }: Omit<Row, 'key'>) {
  const diff = current - previous;
  const pct = pctChange(current, previous);
  // Spending going up is the bad direction here.
  const tone = diff > 0 ? 'tone-bad' : diff < 0 ? 'tone-good' : 'tone-neutral';
  return (
    <tr>
      <td className="ellipsis" title={name}>
        {color !== undefined && (
          <span className="swatch" style={{ background: color || 'var(--muted)' }} aria-hidden="true" />
        )}{' '}
        {name}
      </td>
      <td className="num">{current ? money(current) : '—'}</td>
      <td className="num">{previous ? money(previous) : '—'}</td>
      <td className={`num ${tone}`}>
        {diff === 0 ? '—' : `${diff > 0 ? '+' : '−'}${money(Math.abs(diff))}`}
        {pct !== null && diff !== 0 && (
          <span className="hint">
            {' '}
            ({pct > 0 ? '+' : ''}
            {pct}%)
          </span>
        )}
      </td>
    </tr>
  );
}

export default function CompareTable({ data, loading }: Props) {
  const [tab, setTab] = useState<Tab>('categories');
  const rows: Row[] =
    tab === 'categories'
      ? data.categories.map((c) => ({
          key: String(c.category_id ?? 'none'),
          name: c.name || 'Без категории',
          color: c.color,
          current: c.current,
          previous: c.previous,
        }))
      : data.merchants.map((m) => ({ key: m.merchant, name: m.merchant, current: m.current, previous: m.previous }));

  return (
    <div className={loading ? 'is-loading' : undefined}>
      <div className="compare-head">
        <div className="seg" role="tablist">
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'categories'}
            className={tab === 'categories' ? 'seg-btn is-active' : 'seg-btn'}
            onClick={() => setTab('categories')}
          >
            Категории
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'merchants'}
            className={tab === 'merchants' ? 'seg-btn is-active' : 'seg-btn'}
            onClick={() => setTab('merchants')}
          >
            Продавцы
          </button>
        </div>
        <span className="hint">
          {fmtDayFull(data.current.from)} – {fmtDayFull(data.current.to)} против {fmtDayFull(data.previous.from)} –{' '}
          {fmtDayFull(data.previous.to)}
        </span>
      </div>
      <table className="table">
        <thead>
          <tr>
            <th>{tab === 'categories' ? 'Категория' : 'Продавец'}</th>
            <th className="num">Сейчас</th>
            <th className="num">Раньше</th>
            <th className="num">Изменение</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <DeltaRow key={r.key} name={r.name} color={r.color} current={r.current} previous={r.previous} />
          ))}
          {rows.length === 0 && (
            <tr>
              <td colSpan={4} className="empty">
                Нет данных для сравнения
              </td>
            </tr>
          )}
        </tbody>
        <tfoot>
          <DeltaRow name="Итого" current={data.current_totals.expense} previous={data.previous_totals.expense} />
        </tfoot>
      </table>
    </div>
  );
}
