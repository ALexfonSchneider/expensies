import type { Granularity } from '../api';
import { GRANULARITY_LABELS, PRESETS, type PeriodState } from '../period';

interface Props {
  state: PeriodState;
  showGranularity?: boolean;
}

const GRANULARITIES: Granularity[] = ['day', 'week', 'month'];

export default function PeriodBar({ state, showGranularity = true }: Props) {
  return (
    <div className="filters" role="region" aria-label="Фильтры">
      <div className="seg" role="group" aria-label="Период">
        {PRESETS.map((p) => (
          <button
            key={p.key}
            type="button"
            className={state.preset === p.key ? 'seg-btn is-active' : 'seg-btn'}
            onClick={() => state.setPreset(p.key)}
          >
            {p.label}
          </button>
        ))}
      </div>
      <div className="range" aria-label="Свой диапазон">
        <input
          type="date"
          value={state.period.from}
          max={state.period.to}
          onChange={(e) => state.setCustom(e.target.value, state.period.to)}
        />
        <span className="range-dash">–</span>
        <input
          type="date"
          value={state.period.to}
          min={state.period.from}
          onChange={(e) => state.setCustom(state.period.from, e.target.value)}
        />
      </div>
      {showGranularity && (
        <div className="seg" role="group" aria-label="Гранулярность">
          {GRANULARITIES.map((g) => (
            <button
              key={g}
              type="button"
              className={state.granularity === g ? 'seg-btn is-active' : 'seg-btn'}
              onClick={() => state.setGranularity(g)}
            >
              {GRANULARITY_LABELS[g]}
            </button>
          ))}
        </div>
      )}
      <label className="check">
        <input
          type="checkbox"
          checked={state.includeTransfers}
          onChange={(e) => state.setIncludeTransfers(e.target.checked)}
        />
        Учитывать переводы
      </label>
    </div>
  );
}
