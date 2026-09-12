import type { CategoryTotal } from '../api';
import { int, money, plural } from '../format';

interface Props {
  items: CategoryTotal[];
  loading?: boolean;
  onSelect?: (categoryId: number | null) => void;
}

/**
 * Ranked horizontal bars rendered in plain HTML: one series, one hue, the
 * category name and value are the labels. The category swatch beside the
 * name is the identity token used everywhere else in the UI.
 */
export default function CategoryBars({ items, loading, onSelect }: Props) {
  const spending = items.filter((c) => c.expense > 0);
  const max = spending.reduce((m, c) => Math.max(m, c.expense), 0);
  const total = spending.reduce((s, c) => s + c.expense, 0);

  if (spending.length === 0) {
    return <div className={loading ? 'empty is-loading' : 'empty'}>Нет расходов за период</div>;
  }

  return (
    <div className={loading ? 'bars is-loading' : 'bars'}>
      {spending.map((c) => {
        const name = c.name || 'Без категории';
        const share = total > 0 ? Math.round((c.expense / total) * 100) : 0;
        const width = max > 0 ? (c.expense / max) * 100 : 0;
        const clickable = Boolean(onSelect);
        return (
          <div
            key={c.category_id ?? 'none'}
            className={clickable ? 'bar-row is-clickable' : 'bar-row'}
            role={clickable ? 'button' : undefined}
            tabIndex={clickable ? 0 : undefined}
            onClick={() => onSelect?.(c.category_id)}
            onKeyDown={(e) => {
              if (clickable && (e.key === 'Enter' || e.key === ' ')) {
                e.preventDefault();
                onSelect?.(c.category_id);
              }
            }}
            title={`${name}: ${money(c.expense)}, ${int(c.count)} ${plural(c.count, ['операция', 'операции', 'операций'])}`}
          >
            <div className="bar-name">
              <span className="swatch" style={{ background: c.color || 'var(--muted)' }} aria-hidden="true" />
              <span className="bar-name-text">{name}</span>
            </div>
            <div className="bar-track">
              <div className="bar-fill" style={{ width: `${width}%` }} />
            </div>
            <div className="bar-value">{money(c.expense)}</div>
            <div className="bar-share">{share}%</div>
          </div>
        );
      })}
    </div>
  );
}
