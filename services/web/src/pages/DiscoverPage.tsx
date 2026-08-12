import { useInfiniteQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { EmptyState } from "../components/ui/EmptyState";
import { Skeleton } from "../components/ui/Skeleton";
import { listAudios, trackKeys } from "../services/trackService";
import type { AudioRecord } from "../types/audio";
import { formatTime } from "../utils/time";

function CatalogRow({ record }: { record: AudioRecord }) {
  const duration = Math.max(0, Math.floor((record.duration_ms ?? 0) / 1000));
  return (
    <div className="track-row">
      <span className="track-row__index" aria-hidden="true" />
      <span aria-hidden="true" />
      <div>
        <Link to={`/track/${record.id}`}>{record.title}</Link>
        <span>Creator profiles arrive with the metadata API</span>
      </div>
      <span className="track-row__category">Public</span>
      <span className="track-row__date">{new Date(record.created_at).toLocaleDateString()}</span>
      <span className="track-row__duration">{duration ? formatTime(duration) : "—"}</span>
      <span aria-hidden="true" />
    </div>
  );
}

// 공개 카탈로그다. 소유자를 가리지 않고, 서버가 public + READY 로 좁혀 준다.
function Catalog() {
  const query = useInfiniteQuery({
    queryKey: trackKeys.publicList(),
    queryFn: ({ pageParam }) => listAudios({ scope: "public", cursor: pageParam }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });

  if (query.isPending) {
    return (
      <div aria-busy="true">
        <Skeleton className="skeleton--row" />
        <Skeleton className="skeleton--row" />
        <Skeleton className="skeleton--row" />
      </div>
    );
  }

  if (query.isError) {
    return (
      <EmptyState
        eyebrow="REQUEST FAILED"
        title="Could not load the catalog"
        description={query.error instanceof Error ? query.error.message : "The catalog request failed."}
      />
    );
  }

  const records = query.data.pages.flatMap((page) => page.items);
  if (records.length === 0) {
    return (
      <EmptyState
        title="The catalog is quiet"
        description="Public tracks appear here after their audio processing completes."
        action={(
          <Link className="button button--primary button--md" to="/upload">
            Upload a track
          </Link>
        )}
      />
    );
  }

  return (
    <div>
      {records.map((record) => (
        <CatalogRow key={record.id} record={record} />
      ))}
      {query.hasNextPage ? (
        <button
          className="button button--md"
          onClick={() => query.fetchNextPage()}
          disabled={query.isFetchingNextPage}
        >
          {query.isFetchingNextPage ? "Loading…" : "Load more"}
        </button>
      ) : null}
    </div>
  );
}

export function DiscoverPage() {
  return (
    <div className="page-stack">
      <header className="page-hero page-hero--discover">
        <div>
          <p className="eyebrow">DISCOVER INDEPENDENT AUDIO</p>
          <h1>
            Find a signal
            <br />
            worth following.
          </h1>
          <p>
            The catalog will combine music, podcasts, mixes, ambient work, and field recordings
            without flattening them into one feed.
          </p>
        </div>
        <div className="discovery-orbit" aria-hidden="true">
          <span />
          <span />
          <span />
        </div>
      </header>
      <section className="section-block">
        <div className="section-heading">
          <div>
            <p className="eyebrow">CATALOG</p>
            <h2>Recently published</h2>
          </div>
        </div>
        <Catalog />
      </section>
      <section className="discovery-pillars" aria-label="Future discovery dimensions">
        <article>
          <span>01</span>
          <h3>By signal</h3>
          <p>Mood, texture, and format filters built on real metadata.</p>
        </article>
        <article>
          <span>02</span>
          <h3>By creator</h3>
          <p>Follow independent creators and hear what they publish next.</p>
        </article>
        <article>
          <span>03</span>
          <h3>By collection</h3>
          <p>Editorial playlists without fabricated popularity rankings.</p>
        </article>
      </section>
    </div>
  );
}
