import { Heart, ListPlus, MoreHorizontal, Share2 } from "lucide-react";

export function TrackActions() {
  return <div className="inline-actions"><button className="icon-button" disabled aria-label="Like"><Heart /></button><button className="icon-button" disabled aria-label="Add to playlist"><ListPlus /></button><button className="icon-button" disabled aria-label="Share"><Share2 /></button><button className="icon-button" disabled aria-label="More"><MoreHorizontal /></button></div>;
}
