import { useParams } from "react-router-dom";
import { EmptyState } from "../components/ui/EmptyState";
import { playlistServiceStatus } from "../services/playlistService";

export function PlaylistPage() { const { playlistId } = useParams(); return <div className="page-stack"><header className="page-title"><p className="eyebrow">PLAYLIST · {playlistId}</p><h1>Playlist</h1><p>Playlists will support ordered playback, owner edits, saved collections, and sharing.</p></header><EmptyState title="This playlist is not available yet" description={playlistServiceStatus.reason} /></div>; }
