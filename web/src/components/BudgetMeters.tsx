import { Link } from 'react-router-dom';

import type { BudgetReport, BudgetStatus } from '../api';
import { money } from '../format';

interface Props {
  report: BudgetReport;
  loading?: boolean;
}

type State = 'ok' | 'ahead' | 'over';

function stateOf(b: BudgetStatus): State {
  if (b.limit <= 0) return 'ok';
  if (b.spent >= b.limit) return 'over';
  if (b.spent > b.pace_limit) return 'ahead';
  return 'ok';
}

// Status is carried by icon + text as well as color, so a meter never
// relies on hue alone.
function statusText(b: BudgetStatus, state: State): string {
  switch (state) {
    case 'over':
      return `✕ превышен на ${money(b.spent - b.limit)}`;
    case 'ahead':
      return `▲ быстрее плана на ${money(b.spent - b.pace_limit)}`;
    default:
      return `✓ в плане, остаток ${money(b.limit - b.spent)}`;
  }
}

export default function BudgetMeters({ report, loading }: Props) {
  const rows = [...(report.total.limit > 0 ? [report.total] : []), ...report.items];
  const inProgress = report.days_elapsed > 0 && report.days_elapsed < report.days_in_month;

  if (rows.length === 0) {
    return (
      <div className="empty">
        Лимиты пока не заданы. <Link to="/categories">Задать бюджеты</Link>
      </div>
    );
  }

  return (
    <div className={loading ? 'meters is-loading' : 'meters'}>
      {rows.map((b) => {
        const state = stateOf(b);
        const pct = Math.min(100, (b.spent / b.limit) * 100);
        const pace = Math.min(100, (b.pace_limit / b.limit) * 100);
        const tone = state === 'over' ? 'tone-bad' : state === 'ahead' ? 'tone-warn' : 'tone-good';
        return (
          <div className="meter" key={b.budget_id}>
            <div className="meter-head">
              <span className="bar-name">
                <span className="swatch" style={{ background: b.color || 'var(--muted)' }} aria-hidden="true" />
                <span className="bar-name-text">{b.name}</span>
              </span>
              <span className="meter-values">
                <strong>{money(b.spent)}</strong> <span className="hint">из {money(b.limit)}</span>
              </span>
            </div>
            <div
              className="meter-track"
              role="progressbar"
              aria-valuemin={0}
              aria-valuemax={b.limit}
              aria-valuenow={Math.min(b.spent, b.limit)}
              aria-label={`${b.name}: ${money(b.spent)} из ${money(b.limit)}`}
            >
              <div className={`meter-fill is-${state}`} style={{ width: `${pct}%` }} />
              {inProgress && <div className="meter-pace" style={{ left: `${pace}%` }} title="Ожидаемый темп на сегодня" />}
            </div>
            <div className={`meter-status ${tone}`}>
              {statusText(b, state)}
              {inProgress && b.forecast > 0 && <span className="hint"> · прогноз {money(b.forecast)}</span>}
            </div>
          </div>
        );
      })}
    </div>
  );
}
