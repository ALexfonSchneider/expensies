import { useCallback, useEffect, useState, type FormEvent } from 'react';

import { api, errorMessage, type Budget, type Category, type CategoryInput, type Rule, type RuleInput } from '../api';
import { FIELD_LABELS, int, OP_LABELS } from '../format';

const EMPTY_CATEGORY: CategoryInput = { name: '', color: '#2a78d6', is_transfer: false, sort_order: 500 };
const EMPTY_RULE: RuleInput = { category_id: 0, field: 'merchant', op: 'contains', value: '', priority: 50 };

interface BudgetInputProps {
  budget?: Budget;
  onSave: (kopecks: number) => void;
  onDelete: (budget: Budget) => void;
}

// Rubles in, kopecks out. An emptied field deletes the limit; commit on
// blur or Enter so the table does not fire a request per keystroke.
function BudgetInput({ budget, onSave, onDelete }: BudgetInputProps) {
  const [value, setValue] = useState(budget ? String(budget.amount / 100) : '');
  const budgetId = budget?.id;
  const budgetAmount = budget?.amount;
  useEffect(() => {
    setValue(budgetAmount !== undefined ? String(budgetAmount / 100) : '');
  }, [budgetId, budgetAmount]);

  const commit = () => {
    const v = value.trim().replace(',', '.');
    if (v === '') {
      if (budget) onDelete(budget);
      return;
    }
    const rubles = Number(v);
    if (!Number.isFinite(rubles) || rubles <= 0) return;
    const kopecks = Math.round(rubles * 100);
    if (budget && kopecks === budget.amount) return;
    onSave(kopecks);
  };

  return (
    <input
      type="number"
      min={0}
      step={100}
      className="short budget-input"
      placeholder="—"
      value={value}
      onChange={(e) => setValue(e.target.value)}
      onBlur={commit}
      onKeyDown={(e) => {
        if (e.key === 'Enter') (e.target as HTMLInputElement).blur();
      }}
      aria-label="Лимит в месяц, рублей"
    />
  );
}

export default function Categories() {
  const [categories, setCategories] = useState<Category[]>([]);
  const [rules, setRules] = useState<Rule[]>([]);
  const [budgets, setBudgets] = useState<Budget[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [editing, setEditing] = useState<Category | null>(null);
  const [newCategory, setNewCategory] = useState<CategoryInput>(EMPTY_CATEGORY);
  const [newRule, setNewRule] = useState<RuleInput>(EMPTY_RULE);
  const [applying, setApplying] = useState(false);

  const reload = useCallback(() => {
    Promise.all([api.categories.list(), api.rules.list(), api.budgets.list()])
      .then(([c, r, b]) => {
        setCategories(c.items);
        setRules(r.items);
        setBudgets(b.items);
      })
      .catch((e: unknown) => setError(errorMessage(e)));
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  const fail = (e: unknown) => setError(errorMessage(e));

  const submitCategory = (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    api.categories
      .create(newCategory)
      .then(() => {
        setNewCategory(EMPTY_CATEGORY);
        reload();
      })
      .catch(fail);
  };

  const saveEditing = (e: FormEvent) => {
    e.preventDefault();
    if (!editing) return;
    setError(null);
    const { id, ...input } = editing;
    api.categories
      .update(id, input)
      .then(() => {
        setEditing(null);
        reload();
      })
      .catch(fail);
  };

  const removeCategory = (c: Category) => {
    if (!window.confirm(`Удалить категорию «${c.name}»? Её операции останутся без категории, правила будут удалены.`)) return;
    api.categories.remove(c.id).then(reload).catch(fail);
  };

  const saveBudget = (categoryId: number | null, kopecks: number) => {
    setError(null);
    api.budgets.set(categoryId, kopecks).then(reload).catch(fail);
  };

  const deleteBudget = (b: Budget) => {
    setError(null);
    api.budgets.remove(b.id).then(reload).catch(fail);
  };

  const submitRule = (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    api.rules
      .create(newRule)
      .then(() => {
        setNewRule({ ...EMPTY_RULE, category_id: newRule.category_id });
        reload();
      })
      .catch(fail);
  };

  const removeRule = (r: Rule) => {
    api.rules.remove(r.id).then(reload).catch(fail);
  };

  const applyRules = () => {
    setApplying(true);
    setNotice(null);
    setError(null);
    api.rules
      .apply()
      .then((r) => setNotice(`Правила применены, изменено операций: ${int(r.updated)}`))
      .catch(fail)
      .finally(() => setApplying(false));
  };

  const byId = new Map(categories.map((c) => [c.id, c]));
  const budgetByCategory = new Map<number | null, Budget>(budgets.map((b) => [b.category_id, b]));
  const totalBudget = budgetByCategory.get(null);

  return (
    <div className="page">
      <h1>Категории и правила</h1>
      {error && <div className="alert">{error}</div>}
      {notice && <div className="alert alert-ok">{notice}</div>}

      <div className="grid-2">
        <section className="card">
          <div className="card-head">
            <h2>Категории и бюджеты</h2>
            <label className="check">
              Общий лимит в месяц, ₽
              <BudgetInput budget={totalBudget} onSave={(k) => saveBudget(null, k)} onDelete={deleteBudget} />
            </label>
          </div>
          <p className="hint">
            Бюджет задаётся в рублях на календарный месяц. Пустое поле убирает лимит. Переводы в бюджеты не входят.
          </p>
          <table className="table">
            <thead>
              <tr>
                <th>Название</th>
                <th className="num">Лимит, ₽/мес</th>
                <th>Переводы</th>
                <th className="num">Порядок</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {categories.map((c) =>
                editing && editing.id === c.id ? (
                  <tr key={c.id}>
                    <td colSpan={5}>
                      <form className="inline-form" onSubmit={saveEditing}>
                        <input
                          type="color"
                          value={editing.color}
                          onChange={(e) => setEditing({ ...editing, color: e.target.value })}
                          aria-label="Цвет"
                        />
                        <input
                          value={editing.name}
                          onChange={(e) => setEditing({ ...editing, name: e.target.value })}
                          aria-label="Название"
                          required
                        />
                        <label className="check">
                          <input
                            type="checkbox"
                            checked={editing.is_transfer}
                            onChange={(e) => setEditing({ ...editing, is_transfer: e.target.checked })}
                          />
                          перевод
                        </label>
                        <input
                          type="number"
                          className="short"
                          value={editing.sort_order}
                          onChange={(e) => setEditing({ ...editing, sort_order: Number(e.target.value) })}
                          aria-label="Порядок"
                        />
                        <button type="submit" className="btn btn-primary">
                          Сохранить
                        </button>
                        <button type="button" className="btn" onClick={() => setEditing(null)}>
                          Отмена
                        </button>
                      </form>
                    </td>
                  </tr>
                ) : (
                  <tr key={c.id}>
                    <td>
                      <span className="swatch" style={{ background: c.color }} aria-hidden="true" /> {c.name}
                    </td>
                    <td className="num">
                      {c.is_transfer ? (
                        <span className="hint">—</span>
                      ) : (
                        <BudgetInput
                          budget={budgetByCategory.get(c.id)}
                          onSave={(k) => saveBudget(c.id, k)}
                          onDelete={deleteBudget}
                        />
                      )}
                    </td>
                    <td>{c.is_transfer ? 'да' : ''}</td>
                    <td className="num">{c.sort_order}</td>
                    <td className="num nowrap">
                      <button type="button" className="btn btn-small" onClick={() => setEditing(c)}>
                        Изменить
                      </button>{' '}
                      <button type="button" className="btn btn-small btn-danger" onClick={() => removeCategory(c)}>
                        Удалить
                      </button>
                    </td>
                  </tr>
                ),
              )}
            </tbody>
          </table>
          <form className="inline-form" onSubmit={submitCategory}>
            <input
              type="color"
              value={newCategory.color}
              onChange={(e) => setNewCategory({ ...newCategory, color: e.target.value })}
              aria-label="Цвет"
            />
            <input
              placeholder="Новая категория"
              value={newCategory.name}
              onChange={(e) => setNewCategory({ ...newCategory, name: e.target.value })}
              required
            />
            <label className="check">
              <input
                type="checkbox"
                checked={newCategory.is_transfer}
                onChange={(e) => setNewCategory({ ...newCategory, is_transfer: e.target.checked })}
              />
              перевод
            </label>
            <button type="submit" className="btn btn-primary">
              Добавить
            </button>
          </form>
        </section>

        <section className="card">
          <div className="card-head">
            <h2>Правила</h2>
            <button type="button" className="btn" disabled={applying} onClick={applyRules}>
              {applying ? 'Применяем…' : 'Применить ко всем операциям'}
            </button>
          </div>
          <p className="hint">
            Правила проверяются по возрастанию приоритета, первое совпавшее назначает категорию. Сравнение без учёта
            регистра. Категории, выставленные вручную, правила не трогают.
          </p>
          <table className="table">
            <thead>
              <tr>
                <th className="num">Приор.</th>
                <th>Поле</th>
                <th>Условие</th>
                <th>Значение</th>
                <th>Категория</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {rules.map((r) => (
                <tr key={r.id}>
                  <td className="num">{r.priority}</td>
                  <td>{FIELD_LABELS[r.field] ?? r.field}</td>
                  <td>{OP_LABELS[r.op] ?? r.op}</td>
                  <td>
                    <code>{r.value}</code>
                  </td>
                  <td>
                    <span className="swatch" style={{ background: byId.get(r.category_id)?.color ?? 'transparent' }} aria-hidden="true" />{' '}
                    {byId.get(r.category_id)?.name ?? r.category_id}
                  </td>
                  <td className="num">
                    <button type="button" className="btn btn-small btn-danger" onClick={() => removeRule(r)}>
                      Удалить
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <form className="inline-form" onSubmit={submitRule}>
            <input
              type="number"
              className="short"
              value={newRule.priority}
              onChange={(e) => setNewRule({ ...newRule, priority: Number(e.target.value) })}
              aria-label="Приоритет"
            />
            <select value={newRule.field} onChange={(e) => setNewRule({ ...newRule, field: e.target.value })} aria-label="Поле">
              {Object.entries(FIELD_LABELS).map(([k, v]) => (
                <option key={k} value={k}>
                  {v}
                </option>
              ))}
            </select>
            <select value={newRule.op} onChange={(e) => setNewRule({ ...newRule, op: e.target.value })} aria-label="Условие">
              {Object.entries(OP_LABELS).map(([k, v]) => (
                <option key={k} value={k}>
                  {v}
                </option>
              ))}
            </select>
            <input
              placeholder="Значение"
              value={newRule.value}
              onChange={(e) => setNewRule({ ...newRule, value: e.target.value })}
              required
            />
            <select
              value={newRule.category_id || ''}
              onChange={(e) => setNewRule({ ...newRule, category_id: Number(e.target.value) })}
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
            <button type="submit" className="btn btn-primary">
              Добавить правило
            </button>
          </form>
        </section>
      </div>
    </div>
  );
}
