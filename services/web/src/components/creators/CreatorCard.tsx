import type { CreatorSummary } from "../../types/audio";

export function CreatorCard({ creator }: { creator: CreatorSummary }) {
  return <article className="creator-card"><div className="avatar">{creator.displayName.slice(0, 1).toUpperCase()}</div><strong>{creator.displayName}</strong><span>@{creator.username}</span></article>;
}
