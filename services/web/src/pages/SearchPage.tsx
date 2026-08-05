import { Search } from "lucide-react";
import { FormEvent, useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { EmptyState } from "../components/ui/EmptyState";
import { getSearchCapability } from "../services/searchService";

export function SearchPage() {
  const [params, setParams] = useSearchParams();
  const [query, setQuery] = useState(params.get("q") ?? "");
  const capability = getSearchCapability();
  useEffect(() => setQuery(params.get("q") ?? ""), [params]);
  const submit = (event: FormEvent) => { event.preventDefault(); const value = query.trim(); setParams(value ? { q: value } : {}); };
  return <div className="page-stack"><header className="page-title"><p className="eyebrow">SEARCH</p><h1>Trace a sound.</h1><p>Search will query tracks, creators, playlists, genres, and moods through one debounced API boundary.</p></header><form className="search-page__form" onSubmit={submit}><Search aria-hidden="true" /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search tracks, creators, genres, or moods" aria-label="Search query" /><button className="button button--primary button--md">Search</button></form><div className="tab-list" role="tablist">{["All", "Tracks", "Creators", "Playlists", "Genres"].map((tab, index) => <button key={tab} role="tab" aria-selected={index === 0} disabled={index !== 0}>{tab}</button>)}</div><EmptyState eyebrow="SEARCH API PENDING" title={params.get("q") ? `No live results for “${params.get("q")}”` : "Start with a title, creator, genre, or mood"} description={capability.reason} /></div>;
}
