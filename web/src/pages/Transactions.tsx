import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { useSearchParams } from 'react-router-dom';

import {
  api,
  errorMessage,
  type Category,
  type DataRange,
  type Direction,
  type Receipt,
  type RuleInput,
  type Transaction,
  type TransactionPatch,
} from '../api';
import PeriodBar from '../components/PeriodBar';
import ReceiptItems from '../components/ReceiptItems';
import { FIELD_LABELS, fmtDateTime, int, KIND_LABELS, OP_LABELS, plural, signedMoney } from '../format';
import { usePeriodState } from '../period';

const PAGE = 50;

type Editor = { id: number; kind: 'note' | 'rule' | 'receipt' } | null;

function ReceiptForTransaction({ txId, onError }: { txId: number; onError: (e: unknown) => void }) {
  const [receipts, setReceipts] = useState<Receipt[] | null>(null);
  useEffect(() => {
    api.receipts
      .forTransaction(txId)
      .then((r) => setReceipts(r.items))
      .catch(onError);
  }, [txId, onError]);
  if (!receipts) return <div className="hint">Загружаем чек…</div>;
  if (receipts.length === 0) return <div className="hint">Чек не найден</div>;
  return (
    <div className="receipts-stack">
      {receipts.map((r) => (
        <ReceiptItems key={r.id} receipt={r} />
      ))}
    </div>
  );
}

interface NoteEditorProps {
  tx: Transaction;
  onSave: (note: string) => void;
  onCancel: () => void;
}

function NoteEditor({ tx, onSave, onCancel }: NoteEditorProps) {
  const [value, setValue] = useState(tx.note);
  return (
    <form
      className="inline-form"
      onSubmit={(e: FormEvent) => {
        e.preventDefault();
        onSave(value.trim());
      }}
    >
      <input
        autoFocus
        className="grow"
        placeholder="Заметка к операции"
        value={value}
        onChange={(e) => setValue(e.target.value)}
      />
      <button type="submit" className="btn btn-primary">
        Сохранить
      </button>
      <button type="button" className="btn" onClick={onCancel}>
        Отмена
      </button>
    </form>
  );
}

interface RuleEditorProps {
  tx: Transaction;
  categories: Category[];
  onCreated: (updated: number) => void;
  onError: (e: unknown) => void;
  onCancel: () => void;
}

// Seeds the rule with the most specific attribute the line has, so one
// click covers every future operation at the same merchant.
function seedRule(tx: Transaction): RuleInput {
  let field = 'description';
  let value = tx.description;
  if (tx.merchant) {
    field = 'merchant';
    value = tx.merchant;
  } else if (tx.counterparty) {
    field = 'counterparty';
    value = tx.counterparty;
  }
  return { category_id: tx.category_id ?? 0, field, op: 'contains', value, priority: 50 };
}

function RuleEditor({ tx, categories, onCreated, onError, onCancel }: RuleEditorProps) {
  const [rule, setRule] = useState<RuleInput>(() => seedRule(tx));
  const [busy, setBusy] = useState(false);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (!rule.category_id || !rule.value.trim()) return;
    setBusy(true);
    api.rules
      .create({ ...rule, value: rule.value.trim() })
      .then(() => api.rules.apply())
      .then((r) => onCreated(r.updated))
      .catch(onError)
      .finally(() => setBusy(false));
  };

  return (
    <form className="inline-form" onSubmit={submit}>
      <span className="hint">Правило:</span>
      <select value={rule.field} onChange={(e) => setRule({ ...rule, field: e.target.value })} aria-label="Поле">
        {Object.entries(FIELD_LABELS).map(([k, v]) => (
          <option key={k} value={k}>
            {v}
          </option>
        ))}
      </select>
      <select value={rule.op} onChange={(e) => setRule({ ...rule, op: e.target.value })} aria-label="Условие">
        {Object.entries(OP_LABELS).map(([k, v]) => (
          <option key={k} value={k}>
            {v}
          </option>
        ))}
      </select>
      <input
        className="grow"
        value={rule.value}
        onChange={(e) => setRule({ ...rule, value: e.target.value })}
        aria-label="Значение"
        required
      />
      <span className="hint">→</span>
      <select
        value={rule.category_id || ''}
        onChange={(e) => setRule({ ...rule, category_id: Number(e.target.value) })}
        aria-label="Категория"
        required
      >
        <option value="">Категория…</option>
        {categories.map((c) => (
          <option key={c.id} value={c.id}>
            {c.name}
          </option>
        ))}
      </select>
      <input
        type="number"
        className="short"
        value={rule.priority}
        onChange={(e) => setRule({ ...rule, priority: Number(e.target.value) })}
        aria-label="Приоритет"
        title="Приоритет: меньше = раньше"
      />
      <button type="submit" className="btn btn-primary" disabled={busy}>
        {busy ? 'Применяем…' : 'Создать и применить'}
      </button>
      <button type="button" className="btn" onClick={onCancel}>
        Отмена
      </button>
    </form>
  );
}

export default function Transactions() {
  const [sp, setSp] = useSearchParams();
  const [range, setRange] = useState<DataRange | null>(null);
  const state = usePeriodState(range);

  const [categories, setCategories] = useState<Category[]>([]);
  const [items, setItems] = useState<Transaction[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [editor, setEditor] = useState<Editor>(null);
  const [query, setQuery] = useState(sp.get('q') ?? '');

  const idParam = sp.get('id') ?? '';
  const categoryParam = sp.get('category') ?? '';
  const uncategorized = sp.get('uncategorized') === '1';
  const direction = (sp.get('direction') ?? '') as Direction | '';
  const q = sp.get('q') ?? '';

  useEffect(() => {
    api.analytics.range().then(setRange).catch(() => setRange(null));
    api.categories.list().then((r) => setCategories(r.items)).catch((e: unknown) => setError(errorMessage(e)));
  }, []);

  // Debounce the search box into the URL so typing does not fire a request
  // per keystroke and the final query survives a refresh.
  useEffect(() => {
    const t = setTimeout(() => {
      if (query !== q) {
        setSp(
          (prev) => {
            const next = new URLSearchParams(prev);
            if (query) next.set('q', query);
            else next.delete('q');
            return next;
          },
          { replace: true },
        );
      }
    }, 300);
    return () => clearTimeout(t);
  }, [query, q, setSp]);

  const setParam = useCallback(
    (key: string, value: string | null) => {
      setSp(
        (prev) => {
          const next = new URLSearchParams(prev);
          if (value === null || value === '') next.delete(key);
          else next.set(key, value);
          return next;
        },
        { replace: true },
      );
    },
    [setSp],
  );

  const { from, to } = state.period;
  const includeTransfers = state.includeTransfers;

  const load = useCallback(
    (offset: number) => {
      setLoading(true);
      setError(null);
      return api.transactions
        .list({
          id: idParam ? Number(idParam) : undefined,
          from,
          to,
          include_transfers: idParam ? true : includeTransfers,
          category_id: categoryParam ? Number(categoryParam) : undefined,
          uncategorized,
          direction,
          q,
          limit: PAGE,
          offset,
        })
        .then((r) => {
          setTotal(r.total);
          setItems((prev) => (offset === 0 ? r.items : [...prev, ...r.items]));
          // Arriving from a receipt: show the operation with its receipt open.
          if (idParam && r.items.length === 1 && r.items[0].receipt_id !== null) {
            setEditor({ id: r.items[0].id, kind: 'receipt' });
          }
        })
        .catch((e: unknown) => setError(errorMessage(e)))
        .finally(() => setLoading(false));
    },
    [idParam, from, to, includeTransfers, categoryParam, uncategorized, direction, q],
  );

  useEffect(() => {
    void load(0);
  }, [load]);

  const byId = new Map(categories.map((c) => [c.id, c]));
  const fail = useCallback((e: unknown) => setError(errorMessage(e)), []);

  const patch = (tx: Transaction, body: TransactionPatch) =>
    api.transactions
      .patch(tx.id, body)
      .then((updated) => setItems((prev) => prev.map((t) => (t.id === updated.id ? updated : t))))
      .catch(fail);

  return (
    <div className="page">
      <h1>Операции</h1>
      <PeriodBar state={state} showGranularity={false} />
      <div className="filters">
        <input
          type="search"
          className="search"
          placeholder="Поиск по описанию, продавцу, заметке"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <select
          value={uncategorized ? 'none' : categoryParam}
          onChange={(e) => {
            const v = e.target.value;
            setSp(
              (prev) => {
                const next = new URLSearchParams(prev);
                next.delete('category');
                next.delete('uncategorized');
                if (v === 'none') next.set('uncategorized', '1');
                else if (v) next.set('category', v);
                return next;
              },
              { replace: true },
            );
          }}
        >
          <option value="">Все категории</option>
          <option value="none">Без категории</option>
          {categories.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </select>
        <select value={direction} onChange={(e) => setParam('direction', e.target.value)}>
          <option value="">Расход и приход</option>
          <option value="expense">Только расход</option>
          <option value="income">Только приход</option>
        </select>
        <span className="hint">
          {int(total)} {plural(total, ['операция', 'операции', 'операций'])}
        </span>
      </div>
      {error && <div className="alert">{error}</div>}
      {notice && <div className="alert alert-ok">{notice}</div>}
      {idParam && (
        <div className="alert alert-ok">
          Показана одна операция по чеку.{' '}
          <button type="button" className="btn btn-small" onClick={() => setParam('id', null)}>
            Показать все операции периода
          </button>
        </div>
      )}

      <div className="card">
        <table className={loading ? 'table tx-table is-loading' : 'table tx-table'}>
          <thead>
            <tr>
              <th>Дата</th>
              <th>Операция</th>
              <th>Категория</th>
              <th className="num">Сумма</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {items.map((t) => {
              const cat = t.category_id !== null ? byId.get(t.category_id) : undefined;
              const active = editor && editor.id === t.id ? editor.kind : null;
              return [
                <tr key={t.id} className={t.excluded ? 'is-excluded' : undefined}>
                  <td className="nowrap">{fmtDateTime(t.op_at)}</td>
                  <td>
                    <div className="tx-title">
                      {t.merchant || t.counterparty || t.description}
                      {t.receipt_id !== null && (
                        <button
                          type="button"
                          className="badge badge-receipt"
                          title="Показать состав покупки по чеку"
                          onClick={() => setEditor(active === 'receipt' ? null : { id: t.id, kind: 'receipt' })}
                        >
                          чек
                        </button>
                      )}
                    </div>
                    <div className="tx-sub">
                      {KIND_LABELS[t.kind] ?? t.kind}
                      {t.merchant || t.counterparty ? ` · ${t.description}` : ''}
                      {t.card ? ` · ${t.card}` : ''}
                      {t.source === 'receipt' && (
                        <span className="badge badge-muted" title="Операция построена по чеку, в выписке её пока нет">
                          по чеку, вне выписки
                        </span>
                      )}
                      {t.excluded && <span className="badge badge-muted">не учитывается</span>}
                    </div>
                    {t.note && <div className="tx-note">{t.note}</div>}
                  </td>
                  <td>
                    <div className="cat-cell">
                      <span className="swatch" style={{ background: cat?.color ?? 'transparent' }} aria-hidden="true" />
                      <select
                        aria-label="Категория"
                        value={t.category_id ?? ''}
                        onChange={(e) => void patch(t, { category_id: e.target.value === '' ? null : Number(e.target.value) })}
                      >
                        <option value="">Без категории</option>
                        {categories.map((c) => (
                          <option key={c.id} value={c.id}>
                            {c.name}
                          </option>
                        ))}
                      </select>
                      {t.category_source === 'manual' && <span className="badge">вручную</span>}
                    </div>
                  </td>
                  <td className={t.direction === 'income' ? 'num amount income' : 'num amount'}>
                    {signedMoney(t.amount, t.direction)}
                  </td>
                  <td className="num row-actions">
                    <button
                      type="button"
                      className={active === 'note' ? 'btn btn-small is-active' : 'btn btn-small'}
                      title="Заметка"
                      onClick={() => setEditor(active === 'note' ? null : { id: t.id, kind: 'note' })}
                    >
                      Заметка
                    </button>
                    <button
                      type="button"
                      className={active === 'rule' ? 'btn btn-small is-active' : 'btn btn-small'}
                      title="Создать правило категоризации по этой операции"
                      onClick={() => setEditor(active === 'rule' ? null : { id: t.id, kind: 'rule' })}
                    >
                      Правило
                    </button>
                    <button
                      type="button"
                      className="btn btn-small"
                      title={t.excluded ? 'Снова учитывать в статистике' : 'Не учитывать в статистике'}
                      onClick={() => void patch(t, { excluded: !t.excluded })}
                    >
                      {t.excluded ? 'Учитывать' : 'Исключить'}
                    </button>
                  </td>
                </tr>,
                active && (
                  <tr key={`${t.id}-editor`} className="editor-row">
                    <td colSpan={5}>
                      {active === 'receipt' ? (
                        <ReceiptForTransaction txId={t.id} onError={fail} />
                      ) : active === 'note' ? (
                        <NoteEditor
                          tx={t}
                          onSave={(note) => {
                            void patch(t, { note }).then(() => setEditor(null));
                          }}
                          onCancel={() => setEditor(null)}
                        />
                      ) : (
                        <RuleEditor
                          tx={t}
                          categories={categories}
                          onError={fail}
                          onCancel={() => setEditor(null)}
                          onCreated={(updated) => {
                            setEditor(null);
                            setNotice(`Правило создано, изменено операций: ${int(updated)}`);
                            void load(0);
                          }}
                        />
                      )}
                    </td>
                  </tr>
                ),
              ];
            })}
            {items.length === 0 && !loading && (
              <tr>
                <td colSpan={5} className="empty">
                  Ничего не найдено
                </td>
              </tr>
            )}
          </tbody>
        </table>
        {items.length < total && (
          <div className="table-foot">
            <button type="button" className="btn" disabled={loading} onClick={() => void load(items.length)}>
              Показать ещё ({int(total - items.length)})
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
