import type { ReactNode } from "react";
import { Radio } from "lucide-react";

type EmptyStateProps = {
  eyebrow?: string;
  title: string;
  description: string;
  action?: ReactNode;
};

export function EmptyState({ eyebrow = "NO SIGNAL YET", title, description, action }: EmptyStateProps) {
  return (
    <section className="empty-state" aria-labelledby={`empty-${title.replace(/\s+/g, "-").toLowerCase()}`}>
      <div className="empty-state__icon" aria-hidden="true"><Radio size={22} /></div>
      <p className="eyebrow">{eyebrow}</p>
      <h2 id={`empty-${title.replace(/\s+/g, "-").toLowerCase()}`}>{title}</h2>
      <p>{description}</p>
      {action ? <div className="empty-state__action">{action}</div> : null}
    </section>
  );
}
