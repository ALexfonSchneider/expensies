import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';

import {
  api,
  errorMessage,
  type Bucket,
  type BudgetReport,
  type CategoryTotal,
  type Comparison,
  type DataRange,
  type ItemTotal,
  type MerchantTotal,
  type Summary,
} from '../api';
import BudgetMeters from '../components/BudgetMeters';
import CategoryBars from '../components/CategoryBars';
import CompareTable from '../components/CompareTable';
import PeriodBar from '../components/PeriodBar';
import SeriesChart from '../components/SeriesChart';
import StatTile, { type Delta } from '../components/StatTile';
import { addDays, bucketTitle, fmtDayWithWeekday, int, money, monthLabel, pctChange, plural } from '../format';
import { usePeriodState } from '../period';

function expenseDelta(s: Summary): Delta | null {
  const pct = pctChange(s.expense, s.prev.expense);
  if (pct === null) return null;
  const sign = pct > 0 ? '+' : '';
  const tone: Delta['tone'] = pct > 0 ? 'bad' : pct < 0 ? 'good' : 'neutral';
  return { text: `${sign}${pct}% к предыдущим ${s.days} ${plural(s.days, ['дню', 'дням', 'дням'])}`, tone };
}

function forecastDelta(r: BudgetReport): Delta | null {
  const t = r.total;
  if (t.limit <= 0) return null;
  const diff = t.forecast - t.limit;
  if (diff > 0) return { text: `превысит лимит ${money(t.limit)} на ${money(diff)}`, tone: 'bad' };
  return { text: `в лимит ${money(t.limit)}, запас ${money(-diff)}`, tone: 'good' };
}

export default function Dashboard() {
  const navigate = useNavigate();
  const [range, setRange] = useState<DataRange | null>(null);
  const state = usePeriodState(range);

  // Every drill-down keeps the transfers toggle and replaces the period or
  // the filter, so the operations list opens on exactly the clicked slice.
  const openTransactions = (extra: Record<string, string | null>) => {
    const sp = new URLSearchParams(window.location.search);
    for (const key of ['category', 'uncategorized', 'q']) sp.delete(key);
    for (const [k, v] of Object.entries(extra)) {
      if (v === null) sp.delete(k);
      else sp.set(k, v);
    }
    navigate(`/transactions?${sp.toString()}`);
  };

  const openBucket = (start: string) => {
    let to = start;
    if (state.granularity === 'week') to = addDays(start, 6);
    if (state.granularity === 'month') to = addDays(`${start.slice(0, 7)}-01`, 31).slice(0, 7) + '-01';
    if (state.granularity === 'month') to = addDays(to, -1);
    openTransactions({ preset: 'custom', from: start, to, g: null });
  };

  const [summary, setSummary] = useState<Summary | null>(null);
  const [series, setSeries] = useState<Bucket[]>([]);
  const [categories, setCategories] = useState<CategoryTotal[]>([]);
  const [merchants, setMerchants] = useState<MerchantTotal[]>([]);
  const [report, setReport] = useState<BudgetReport | null>(null);
  const [compare, setCompare] = useState<Comparison | null>(null);
  const [items, setItems] = useState<ItemTotal[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.analytics
      .range()
      .then(setRange)
      .catch((e: unknown) => setError(errorMessage(e)));
  }, []);

  const { from, to } = state.period;
  const granularity = state.granularity;
  const includeTransfers = state.includeTransfers;
  // Budgets are monthly; the month that contains the end of the period is
  // the one the reader is looking at.
  const month = to.slice(0, 7);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    const p = { from, to, include_transfers: includeTransfers };
    Promise.all([
      api.analytics.summary(p),
      api.analytics.series(p, granularity),
      api.analytics.categories(p),
      api.analytics.merchants(p, 10),
      api.budgets.report(month),
      api.analytics.compare(p),
      api.analytics.items(p, 12),
    ])
      .then(([s, se, c, m, r, cmp, it]) => {
        if (cancelled) return;
        setSummary(s);
        setSeries(se.items);
        setCategories(c.items);
        setMerchants(m.items);
        setReport(r);
        setCompare(cmp);
        setItems(it.items);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(errorMessage(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [from, to, granularity, includeTransfers, month]);

  if (range && !range.has_data) {
    return (
      <div className="page">
        <h1>Дашборд</h1>
        <div className="card empty-state">
          <p>Пока нет ни одной операции.</p>
          <Link className="btn btn-primary" to="/statements">
            Загрузить выписку
          </Link>
        </div>
      </div>
    );
  }

  const seriesTotal = series.reduce(
    (acc, b) => ({ expense: acc.expense + b.expense, income: acc.income + b.income, count: acc.count + b.count }),
    { expense: 0, income: 0, count: 0 },
  );
  const monthInProgress = report !== null && report.days_elapsed > 0 && report.days_elapsed < report.days_in_month;

  return (
    <div className="page">
      <h1>Дашборд</h1>
      <PeriodBar state={state} />
      {error && <div className="alert">{error}</div>}

      {summary && (
        <div className={loading ? 'kpis is-loading' : 'kpis'}>
          <StatTile
            hero
            label="Расходы за период"
            value={money(summary.expense)}
            delta={expenseDelta(summary)}
            sub={`${int(summary.count)} ${plural(summary.count, ['операция', 'операции', 'операций'])}`}
          />
          {report && monthInProgress && (
            <StatTile
              label={`Прогноз на ${monthLabel(report.month)}`}
              value={money(report.total.forecast)}
              delta={forecastDelta(report)}
              sub={`потрачено ${money(report.total.spent)} за ${report.days_elapsed} из ${report.days_in_month} дней`}
            />
          )}
          <StatTile
            label="В среднем в день"
            value={money(summary.avg_per_day)}
            sub={`${summary.active_days} из ${summary.days} дней с тратами, ${money(summary.avg_per_active_day)} в такой день`}
          />
          <StatTile
            label="Самый дорогой день"
            value={summary.max_day ? money(summary.max_day.expense) : '—'}
            sub={summary.max_day ? fmtDayWithWeekday(summary.max_day.start) : 'нет трат'}
          />
          <StatTile
            label="Приход"
            value={money(summary.income)}
            sub={includeTransfers ? 'включая переводы и пополнения' : 'кэшбэк и возвраты, без переводов'}
          />
        </div>
      )}

      <section className="card">
        <div className="card-head">
          <h2>Расходы {granularity === 'day' ? 'по дням' : granularity === 'week' ? 'по неделям' : 'по месяцам'}</h2>
          <span className="hint">клик по столбцу открывает операции</span>
        </div>
        <SeriesChart items={series} granularity={granularity} loading={loading} onSelect={openBucket} />
      </section>

      <div className="grid-2">
        <section className="card">
          <div className="card-head">
            <h2>По категориям</h2>
            <span className="hint">клик открывает операции</span>
          </div>
          <CategoryBars
            items={categories}
            loading={loading}
            onSelect={(id) => openTransactions(id === null ? { uncategorized: '1' } : { category: String(id) })}
          />
        </section>

        <section className="card">
          <div className="card-head">
            <h2>Куда уходит больше всего</h2>
          </div>
          {merchants.length === 0 ? (
            <div className="empty">Нет расходов за период</div>
          ) : (
            <table className={loading ? 'table is-loading' : 'table'}>
              <thead>
                <tr>
                  <th>Продавец</th>
                  <th className="num">Сумма</th>
                  <th className="num">Опер.</th>
                </tr>
              </thead>
              <tbody>
                {merchants.map((m) => (
                  <tr key={m.merchant} className="is-clickable" onClick={() => openTransactions({ q: m.merchant })}>
                    <td className="ellipsis" title={`Открыть операции: ${m.merchant}`}>
                      {m.merchant}
                    </td>
                    <td className="num">{money(m.expense)}</td>
                    <td className="num">{int(m.count)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>
      </div>

      <div className="grid-2">
        {report && (
          <section className="card">
            <div className="card-head">
              <h2>Бюджеты на {monthLabel(report.month)}</h2>
              <Link className="hint" to="/categories">
                настроить
              </Link>
            </div>
            <BudgetMeters report={report} loading={loading} />
          </section>
        )}
        {compare && (
          <section className="card">
            <div className="card-head">
              <h2>Сравнение с предыдущим периодом</h2>
            </div>
            <CompareTable data={compare} loading={loading} />
          </section>
        )}
      </div>

      {items.length > 0 && (
        <section className="card">
          <div className="card-head">
            <h2>Что покупалось чаще всего</h2>
            <Link className="hint" to="/receipts">
              чеки
            </Link>
          </div>
          <table className={loading ? 'table is-loading' : 'table'}>
            <thead>
              <tr>
                <th>Позиция</th>
                <th className="num">Сумма</th>
                <th className="num">Раз</th>
              </tr>
            </thead>
            <tbody>
              {items.map((it) => (
                <tr key={it.name}>
                  <td className="ellipsis" title={it.name}>
                    {it.name}
                  </td>
                  <td className="num">{money(it.sum)}</td>
                  <td className="num">{int(it.count)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      )}

      <section className="card">
        <details open={series.length <= 32}>
          <summary className="card-head">
            <h2>Итоги по периодам</h2>
          </summary>
          <table className={loading ? 'table is-loading' : 'table'}>
            <thead>
              <tr>
                <th>Период</th>
                <th className="num">Расход</th>
                <th className="num">Приход</th>
                <th className="num">Опер.</th>
              </tr>
            </thead>
            <tbody>
              {series.map((b) => (
                <tr
                  key={b.start}
                  className={b.count === 0 ? 'is-zero is-clickable' : 'is-clickable'}
                  onClick={() => openBucket(b.start)}
                  title="Открыть операции за период"
                >
                  <td>{bucketTitle(b.start, granularity)}</td>
                  <td className="num">{b.expense ? money(b.expense) : '—'}</td>
                  <td className="num">{b.income ? money(b.income) : '—'}</td>
                  <td className="num">{b.count || '—'}</td>
                </tr>
              ))}
            </tbody>
            <tfoot>
              <tr>
                <td>Итого</td>
                <td className="num">{money(seriesTotal.expense)}</td>
                <td className="num">{money(seriesTotal.income)}</td>
                <td className="num">{int(seriesTotal.count)}</td>
              </tr>
            </tfoot>
          </table>
        </details>
      </section>
    </div>
  );
}
