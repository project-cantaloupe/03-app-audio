import { useParams } from "react-router-dom";
import { EmptyState } from "../components/ui/EmptyState";
import { creatorServiceStatus } from "../services/creatorService";

export function CreatorPage() { const { creatorId } = useParams(); return <div className="page-stack"><header className="page-title"><p className="eyebrow">CREATOR · {creatorId}</p><h1>Creator profile</h1></header><div className="tab-list">{["Tracks", "Albums", "Playlists", "Reposts", "About"].map((tab) => <button key={tab} disabled>{tab}</button>)}</div><EmptyState title="This creator profile is not available yet" description={creatorServiceStatus.reason} /></div>; }
