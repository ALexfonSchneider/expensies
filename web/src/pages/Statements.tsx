import { useCallback, useEffect, useRef, useState, type DragEvent } from 'react';

import { api, errorMessage, type ImportResult, type Statement } from '../api';
import { fmtDateTime, fmtDayFull, int, money } from '../format';

export default function Statements() {
  const [items, setItems] = useState<Statement[]>([]);
  const [result, setResult] = useState<ImportResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [dragging, setDragging] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);

  const reload = useCallback(() => {
    api.statements
      .list()
      .then((r) => setItems(r.items))
      .catch((e: unknown) => setError(errorMessage(e)));
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  const upload = (files: FileList | null) => {
    const file = files?.[0];
    if (!file) return;
    setBusy(true);
    setError(null);
    setResult(null);
    api.statements
      .upload(file)
      .then((r) => {
        setResult(r);
        reload();
      })
      .catch((e: unknown) => setError(errorMessage(e)))
      .finally(() => {
        setBusy(false);
        if (fileInput.current) fileInput.current.value = '';
      });
  };

  const onDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setDragging(false);
    upload(e.dataTransfer.files);
  };

  const remove = (st: Statement) => {
    if (!window.confirm(`Удалить выписку ${st.number} и её ${int(st.tx_count)} операций?`)) return;
    api.statements
      .remove(st.id)
      .then(reload)
      .catch((e: unknown) => setError(errorMessage(e)));
  };

  return (
    <div className="page">
      <h1>Выписки</h1>

      <div
        className={dragging ? 'dropzone is-dragging' : busy ? 'dropzone is-busy' : 'dropzone'}
        onDragOver={(e) => {
          e.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={onDrop}
      >
        <p>
          <strong>Перетащите PDF выписки сюда</strong> или
        </p>
        <button type="button" className="btn btn-primary" disabled={busy} onClick={() => fileInput.current?.click()}>
          {busy ? 'Загружаем…' : 'Выбрать файл'}
        </button>
        <input
          ref={fileInput}
          type="file"
          accept="application/pdf,.pdf"
          hidden
          onChange={(e) => upload(e.target.files)}
        />
        <p className="hint">Выписка Яндекс Банка в PDF. Одни и те же операции второй раз не добавятся.</p>
      </div>

      {error && <div className="alert">{error}</div>}

      {result && (
        <div className="card result">
          <h2>
            Выписка {result.statement.number} за {fmtDayFull(result.statement.period_from)} –{' '}
            {fmtDayFull(result.statement.period_to)}
          </h2>
          <p>
            Добавлено операций: <strong>{int(result.imported)}</strong>
            {result.duplicates > 0 && (
              <>
                , пропущено как дубли: <strong>{int(result.duplicates)}</strong>
              </>
            )}
            . Расход по выписке {money(result.statement.total_expense)}, приход {money(result.statement.total_income)}.
          </p>
          {result.warnings.length > 0 && (
            <div className="alert alert-warn">
              <strong>Предупреждения парсера:</strong>
              <ul>
                {result.warnings.map((w, i) => (
                  <li key={i}>{w}</li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}

      <div className="card">
        <table className="table">
          <thead>
            <tr>
              <th>Номер</th>
              <th>Период</th>
              <th className="num">Операций</th>
              <th className="num">Расход</th>
              <th className="num">Приход</th>
              <th className="num">Остаток на конец</th>
              <th>Загружена</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {items.map((st) => (
              <tr key={st.id}>
                <td className="nowrap">{st.number}</td>
                <td className="nowrap">
                  {fmtDayFull(st.period_from)} – {fmtDayFull(st.period_to)}
                </td>
                <td className="num">{int(st.tx_count)}</td>
                <td className="num">{money(st.total_expense)}</td>
                <td className="num">{money(st.total_income)}</td>
                <td className="num">{money(st.closing_balance)}</td>
                <td className="nowrap">{fmtDateTime(st.uploaded_at)}</td>
                <td className="num">
                  <button type="button" className="btn btn-danger" onClick={() => remove(st)}>
                    Удалить
                  </button>
                </td>
              </tr>
            ))}
            {items.length === 0 && (
              <tr>
                <td colSpan={8} className="empty">
                  Пока ничего не загружено
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
