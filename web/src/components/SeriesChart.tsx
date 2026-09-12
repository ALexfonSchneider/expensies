import type { ReactElement } from 'react';
import { Bar, BarChart, CartesianGrid, LabelList, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';

import type { Bucket, Granularity } from '../api';
import { bucketLabel, bucketTitle, int, money, moneyCompact, plural } from '../format';

interface Props {
  items: Bucket[];
  granularity: Granularity;
  loading?: boolean;
  onSelect?: (start: string) => void;
  /** Chart height in pixels; the default fits the dashboard card. */
  height?: number;
  /** A horizontal threshold such as an average, drawn as a dashed line. */
  reference?: { value: number; label: string };
}

interface TipProps {
  active?: boolean;
  payload?: Array<{ payload: Bucket }>;
  granularity: Granularity;
}

function ChartTip({ active, payload, granularity }: TipProps) {
  if (!active || !payload || payload.length === 0) return null;
  const b = payload[0].payload;
  return (
    <div className="tip">
      <div className="tip-title">{bucketTitle(b.start, granularity)}</div>
      <div className="tip-row">
        <span className="tip-key tip-key-expense" />
        <span className="tip-value">{money(b.expense)}</span>
        <span className="tip-name">расход</span>
      </div>
      <div className="tip-row">
        <span className="tip-key tip-key-income" />
        <span className="tip-value">{money(b.income)}</span>
        <span className="tip-name">приход</span>
      </div>
      <div className="tip-foot">
        {int(b.count)} {plural(b.count, ['операция', 'операции', 'операций'])}
      </div>
    </div>
  );
}

interface LabelRenderProps {
  x?: number | string;
  y?: number | string;
  width?: number | string;
  value?: number | string;
  index?: number;
}

export default function SeriesChart({ items, granularity, loading, onSelect, height = 300, reference }: Props) {
  const handleClick = (data: unknown) => {
    const start = (data as { start?: string } | null)?.start;
    if (start && onSelect) onSelect(start);
  };

  // Only the extreme gets a direct label; the axis and tooltip carry the rest.
  let maxIndex = -1;
  items.forEach((b, i) => {
    if (b.expense > 0 && (maxIndex < 0 || b.expense > items[maxIndex].expense)) maxIndex = i;
  });

  const renderMaxLabel = (props: unknown): ReactElement | null => {
    const { x, y, width, value, index } = props as LabelRenderProps;
    if (index !== maxIndex || typeof value !== 'number' || value <= 0) return null;
    const cx = Number(x) + Number(width) / 2;
    return (
      <text x={cx} y={Number(y) - 6} textAnchor="middle" className="bar-label">
        {moneyCompact(value)}
      </text>
    );
  };

  const empty = items.every((b) => b.expense === 0 && b.income === 0);

  return (
    <div className={loading ? 'chart is-loading' : 'chart'} style={{ height }}>
      {empty && !loading && <div className="chart-empty">Нет операций за выбранный период</div>}
      <ResponsiveContainer width="100%" height="100%">
        <BarChart data={items} margin={{ top: 24, right: 12, left: 0, bottom: 4 }} barCategoryGap="35%">
          <CartesianGrid vertical={false} stroke="var(--grid)" />
          <XAxis
            dataKey="start"
            tickFormatter={(v: string) => bucketLabel(v, granularity)}
            tick={{ fill: 'var(--muted)', fontSize: 12 }}
            axisLine={{ stroke: 'var(--axis)' }}
            tickLine={false}
            minTickGap={28}
            interval="preserveStartEnd"
          />
          <YAxis
            tickFormatter={(v: number) => moneyCompact(v)}
            tick={{ fill: 'var(--muted)', fontSize: 12 }}
            axisLine={false}
            tickLine={false}
            width={72}
          />
          <Tooltip cursor={{ fill: 'var(--wash)' }} content={<ChartTip granularity={granularity} />} />
          {reference && reference.value > 0 && (
            <ReferenceLine
              y={reference.value}
              stroke="var(--text-2)"
              strokeDasharray="4 4"
              label={{ value: reference.label, position: 'insideTopRight', fill: 'var(--text-2)', fontSize: 12 }}
            />
          )}
          <Bar
            dataKey="expense"
            fill="var(--series-1)"
            radius={[4, 4, 0, 0]}
            maxBarSize={24}
            isAnimationActive={false}
            onClick={handleClick}
            className={onSelect ? 'is-clickable' : undefined}
          >
            <LabelList dataKey="expense" content={renderMaxLabel} />
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
