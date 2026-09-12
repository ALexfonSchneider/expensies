export interface Delta {
  text: string;
  tone: 'good' | 'bad' | 'neutral';
}

interface Props {
  label: string;
  value: string;
  sub?: string;
  delta?: Delta | null;
  hero?: boolean;
}

export default function StatTile({ label, value, sub, delta, hero }: Props) {
  return (
    <div className={hero ? 'tile tile-hero' : 'tile'}>
      <div className="tile-label">{label}</div>
      <div className="tile-value">{value}</div>
      {delta && <div className={`tile-delta tone-${delta.tone}`}>{delta.text}</div>}
      {sub && <div className="tile-sub">{sub}</div>}
    </div>
  );
}
