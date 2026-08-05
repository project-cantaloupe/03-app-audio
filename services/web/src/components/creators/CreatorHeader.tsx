import type { CreatorSummary } from "../../types/audio";
import { Button } from "../ui/Button";

export function CreatorHeader({ creator }: { creator: CreatorSummary }) {
  return <header className="creator-header"><div className="avatar avatar--large">{creator.displayName.slice(0, 1).toUpperCase()}</div><div><p className="eyebrow">CREATOR</p><h1>{creator.displayName}</h1><span>@{creator.username}</span></div><Button disabled>Follow</Button></header>;
}
