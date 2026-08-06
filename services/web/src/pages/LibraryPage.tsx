import { useInfiniteQuery } from "@tanstack/react-query";
import { Link, useSearchParams } from "react-router-dom";
import { EmptyState } from "../components/ui/EmptyState";
import { Skeleton } from "../components/ui/Skeleton";
import { listAudios, trackKeys } from "../services/trackService";
import type { AudioRecord, AudioStatus } from "../types/audio";
import { formatTime } from "../utils/time";

const views = ["liked", "playlists", "following", "history", "uploads"];

// 처리 중인 트랙도 목록에 온다. 업로드 직후 상태가 보이지 않으면 사용자는
// 업로드가 실패했다고 판단한다.
const statusLabels: Record<AudioStatus, string> = {
  UPLOAD_PENDING: "Waiting for upload",
  UPLOADED: "Uploaded",
  SCANNING: "Scanning",
  CLEAN: "Scanned",
  QUARANTINED: "Quarantined",
  SCAN_FAILED: "Scan failed",
  QUEUED: "Queued",
  TRANSCODING: "Transcoding",
  READY: "Ready",
  TRANSCODE_FAILED: "Transcode failed",
  DELETED: "Deleted",
};

function UploadRow({ record }: { record: AudioRecord }) {
  const duration = Math.max(0, Math.floor((record.duration_ms ?? 0) / 1000));
  return (
    <div className="track-row">
      <span className="track-row__index" aria-hidden="true" />
      <span aria-hidden="true" />
      <div>
        <Link to={`/track/${record.id}`}>{record.title}</Link>
        <span>{record.visibility}</span>
      </div>
      <span className="track-row__category">{statusLabels[record.status] ?? record.status}</span>
      <span className="track-row__date">{new Date(record.created_at).toLocaleDateString()}</span>
      <span className="track-row__duration">{duration ? formatTime(duration) : "—"}</span>
      <span aria-hidden="true" />
    </div>
  );
}

function Uploads() {
  const query = useInfiniteQuery({
    queryKey: trackKeys.list(),
    queryFn: ({ pageParam }) => listAudios({ cursor: pageParam }),
    initialPageParam: undefined as string | undefined,
    // next_cursor가 없으면 마지막 페이지다.
    getNextPageParam: (page) => page.next_cursor,
  });

  if (query.isPending) {
    return (
      <div className="page-stack" aria-busy="true">
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
        title="Could not load your uploads"
        description={query.error instanceof Error ? query.error.message : "The uploads request failed."}
      />
    );
  }

  const records = query.data.pages.flatMap((page) => page.items);
  if (records.length === 0) {
    return (
      <EmptyState
        title="No uploads yet"
        description="Tracks you upload appear here, including ones still being processed."
        action={
          <Link className="button button--primary button--md" to="/upload">
            Upload a track
          </Link>
        }
      />
    );
  }

  return (
    <div>
      {records.map((record) => (
        <UploadRow key={record.id} record={record} />
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

export function LibraryPage() {
  const [params, setParams] = useSearchParams();
  const current = params.get("view") ?? "liked";
  return (
    <div className="page-stack">
      <header className="page-title">
        <p className="eyebrow">YOUR LIBRARY</p>
        <h1>Sounds worth returning to.</h1>
        <p>
          Uploads are backed by the audio API. Saved tracks and creator relationships remain empty
          until their backend contracts are available.
        </p>
      </header>
      <div className="library-toolbar">
        <div className="tab-list" role="tablist">
          {views.map((view) => (
            <button
              key={view}
              role="tab"
              aria-selected={view === current}
              onClick={() => setParams({ view })}
            >
              {view[0].toUpperCase() + view.slice(1)}
            </button>
          ))}
        </div>
        <input className="input" placeholder="Search within library" disabled aria-label="Search library" />
      </div>
      {current === "uploads" ? (
        <Uploads />
      ) : (
        <EmptyState
          title={`No ${current} here yet`}
          description="This view uses no generated tracks or engagement data. Content will appear after the library API is connected."
        />
      )}
    </div>
  );
}
