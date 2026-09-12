import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';

import { api, errorMessage, type Overview as OverviewData } from '../api';
import SeriesChart from '../components/SeriesChart';
import StatTile, { type Delta } from '../components/StatTile';
import { addDays, fmtDateTime, fmtDayWithWeekday, int, KIND_LABELS, money, monthLabel, pctChange, signedMoney } from '../format';

const SEVERITY_LABEL: Record<string, string> = { bad: 'проблема', warn: 'внимание', info: 'к сведению' };

function monthDelta(m: OverviewData['month']): Delta | null {
  const pct = pctChange(m.spent, m.prev_same_point);
  if (pct === null) return null;
  return {
    text: `${pct > 0 ? '+' : ''}${pct}% к прошлому месяцу на ту же дату (${money(m.prev_same_point)})`,
    tone: pct > 0 ? 'bad' : pct < 0 ? 'good' : 'neutral',
  };
}

function forecastDelta(m: OverviewData['month']): Delta | null {
  if (m.limit <= 0) return { text: 'лимит не задан', tone: 'neutral' };
  const diff = m.forecast - m.limit;
  if (diff > 0) return { text: `превысит лимит ${money(m.limit)} на ${money(diff)}`, tone: 'bad' };
  return { text: `в лимит ${money(m.limit)}, запас ${money(-diff)}`, tone: 'good' };
}

function weekDelta(w: OverviewData['week']): Delta | null {
  if (w.typical_week <= 0) return null;
  const pct = pctChange(w.spent, w.typical_week);
  if (pct === null) return null;
  return {
    text: `${pct > 0 ? '+' : ''}${pct}% к типичной неделе (${money(w.typical_week)}, по ${w.weeks_in_avg} нед.)`,
    tone: pct > 0 ? 'bad' : pct < 0 ? 'good' : 'neutral',
  };
}

export default function Overview() {
  const navigate = useNavigate();
  const [data, setData] = useState<OverviewData | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .overview()
      .then(setData)
      .catch((e: unknown) => setError(errorMessage(e)));
  }, []);

  if (error) {
    return (
      <div className="page">
        <h1>Обзор</h1>
        <div className="alert">{error}</div>
      </div>
    );
  }
  if (!data) {
    return (
      <div className="page">
        <h1>Обзор</h1>
        <div className="hint">Загружаем…</div>
      </div>
    );
  }

  const { month, week, trend, signals, recent } = data;

  return (
    <div className="page">
      <div className="page-head">
        <h1>Обзор</h1>
        <span className="hint">{fmtDayWithWeekday(data.today)}</span>
      </div>

      <div className="kpis">
        <StatTile
          hero
          label={`Потрачено в ${monthLabel(month.month).replace(/^(\S+)/, (m) => m)}`}
          value={money(month.spent)}
          delta={monthDelta(month)}
          sub={`${month.days_elapsed} из ${month.days_in_month} дней, прошлый месяц целиком ${money(month.prev_total)}`}
        />
        <StatTile
          label="Прогноз к концу месяца"
          value={money(month.forecast)}
          delta={forecastDelta(month)}
          sub={month.limit > 0 ? `по плану к сегодня ${money(month.pace_limit)}` : 'задайте лимит в разделе «Категории»'}
        />
        <StatTile
          label="Последние 7 дней"
          value={money(week.spent)}
          delta={weekDelta(week)}
          sub={`${fmtDayWithWeekday(week.from)} – ${fmtDayWithWeekday(week.to)}`}
        />
      </div>

      <section className="card">
        <div className="card-head">
          <h2>Требует внимания</h2>
          <span className="hint">
            {signals.length === 0 ? 'всё спокойно' : `${int(signals.length)} сигнал${signals.length === 1 ? '' : signals.length < 5 ? 'а' : 'ов'}`}
          </span>
        </div>
        {signals.length === 0 ? (
          <div className="empty">Бюджеты в плане, выписка свежая, всё разложено по категориям.</div>
        ) : (
          <ul className="signals">
            {signals.map((s, i) => (
              <li key={i} className={`signal signal-${s.severity}`}>
                <span className="signal-dot" aria-label={SEVERITY_LABEL[s.severity]} />
                <div className="signal-body">
                  <Link to={s.link} className="signal-title">
                    {s.title}
                  </Link>
                  {s.detail && <div className="hint">{s.detail}</div>}
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>

      <div className="grid-2">
        <section className="card">
          <div className="card-head">
            <h2>Последние 7 дней</h2>
            <span className="hint">клик открывает день</span>
          </div>
          <SeriesChart
            items={week.days}
            granularity="day"
            height={220}
            onSelect={(start) => navigate(`/transactions?preset=custom&from=${start}&to=${start}`)}
          />
        </section>
        <section className="card">
          <div className="card-head">
            <h2>Тренд за полгода</h2>
            {trend.average > 0 && <span className="hint">среднее {money(trend.average)} в месяц</span>}
          </div>
          <SeriesChart
            items={trend.months}
            granularity="month"
            height={220}
            reference={trend.average > 0 ? { value: trend.average, label: 'среднее' } : undefined}
            onSelect={(start) => {
              const to = addDays(addDays(`${start.slice(0, 7)}-01`, 31).slice(0, 7) + '-01', -1);
              navigate(`/dashboard?preset=custom&from=${start}&to=${to}`);
            }}
          />
        </section>
      </div>

      <section className="card">
        <div className="card-head">
          <h2>Последние операции</h2>
          <Link className="hint" to="/transactions?preset=month">
            все операции
          </Link>
        </div>
        <table className="table">
          <tbody>
            {recent.map((t) => (
              <tr key={t.id}>
                <td className="nowrap">{fmtDateTime(t.op_at)}</td>
                <td>
                  <div className="tx-title">
                    {t.merchant || t.counterparty || t.description}
                    {t.receipt_id !== null && <span className="badge badge-receipt">чек</span>}
                    {t.source === 'receipt' && <span className="badge badge-muted">по чеку</span>}
                  </div>
                  <div className="tx-sub">{KIND_LABELS[t.kind] ?? t.kind}</div>
                </td>
                <td className={t.direction === 'income' ? 'num amount income' : 'num amount'}>{signedMoney(t.amount, t.direction)}</td>
              </tr>
            ))}
            {recent.length === 0 && (
              <tr>
                <td className="empty">Операций пока нет</td>
              </tr>
            )}
          </tbody>
        </table>
      </section>
    </div>
  );
}
