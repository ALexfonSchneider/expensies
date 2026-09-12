import type { Receipt } from '../api';
import { fmtDateTime, money, shortSeller } from '../format';

interface Props {
  receipt: Receipt;
}

function qty(q: number): string {
  return Number.isInteger(q) ? String(q) : q.toFixed(3).replace(/\.?0+$/, '');
}

export default function ReceiptItems({ receipt }: Props) {
  const items = receipt.items ?? [];
  return (
    <div className="receipt">
      <div className="receipt-head">
        <strong>{shortSeller(receipt.seller_name) || 'Продавец не указан'}</strong>
        {receipt.retail_place && <span className="hint"> · {receipt.retail_place}</span>}
        <span className="hint"> · {fmtDateTime(receipt.issued_at)}</span>
        {receipt.seller_inn && <span className="hint"> · ИНН {receipt.seller_inn}</span>}
      </div>
      {items.length === 0 ? (
        <div className="hint">{receipt.items_error ? 'Состав чека недоступен в архиве' : 'Позиции ещё не загружены'}</div>
      ) : (
        <table className="table receipt-items">
          <thead>
            <tr>
              <th>Позиция</th>
              <th className="num">Цена</th>
              <th className="num">Кол-во</th>
              <th className="num">Сумма</th>
            </tr>
          </thead>
          <tbody>
            {items.map((it) => (
              <tr key={it.position}>
                <td>{it.name}</td>
                <td className="num">{money(it.price)}</td>
                <td className="num">{qty(it.quantity)}</td>
                <td className="num">{money(it.sum)}</td>
              </tr>
            ))}
          </tbody>
          <tfoot>
            <tr>
              <td colSpan={3}>Итого по чеку</td>
              <td className="num">{money(receipt.total)}</td>
            </tr>
          </tfoot>
        </table>
      )}
    </div>
  );
}
