import { useQuery } from "@tanstack/react-query";
import { AlertCircle, Check, FileAudio, RefreshCcw, UploadCloud, X } from "lucide-react";
import { DragEvent, FormEvent, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { Button } from "../components/ui/Button";
import { Input } from "../components/ui/Input";
import { useAuthStore } from "../stores/authStore";
import { useUploadStore, type UploadStage } from "../stores/uploadStore";
import { getAudio, trackKeys } from "../services/trackService";
import { completeUpload, createUpload, resolveContentType, uploadFile } from "../services/uploadService";
import type { AudioRecord, Visibility } from "../types/audio";
import { formatBytes } from "../utils/time";

const maximumBytes = 100 * 1024 * 1024;
const terminalStatuses = new Set(["READY", "QUARANTINED", "SCAN_FAILED", "TRANSCODE_FAILED", "DELETED"]);

const stageIndex: Record<UploadStage, number> = {
  idle: 0,
  preparing: 0,
  uploading: 1,
  verifying: 1,
  processing: 2,
  ready: 4,
  failed: 2,
};

export function UploadPage() {
  const inputRef = useRef<HTMLInputElement>(null);
  const abortRef = useRef<AbortController | null>(null);
  const session = useAuthStore((state) => state.session);
  const authMode = useAuthStore((state) => state.mode);
  const [file, setFile] = useState<File | null>(null);
  const [title, setTitle] = useState("");
  // 기본값은 private다. 공개는 올리는 사람이 명시적으로 고른다.
  const [visibility, setVisibility] = useState<Visibility>("private");
  const [dragging, setDragging] = useState(false);
  const stage = useUploadStore((state) => state.stage);
  const uploadSession = useUploadStore((state) => state.session);
  const record = useUploadStore((state) => state.record);
  const transferred = useUploadStore((state) => state.transferred);
  const total = useUploadStore((state) => state.total);
  const error = useUploadStore((state) => state.error);
  const setStage = useUploadStore((state) => state.setStage);
  const setUploadSession = useUploadStore((state) => state.setSession);
  const setRecord = useUploadStore((state) => state.setRecord);
  const setProgress = useUploadStore((state) => state.setProgress);
  const setError = useUploadStore((state) => state.setError);
  const reset = useUploadStore((state) => state.reset);

  const statusQuery = useQuery({
    queryKey: uploadSession ? trackKeys.detail(uploadSession.audioId) : ["audio", "none"],
    queryFn: () => getAudio(uploadSession!.audioId),
    enabled: Boolean(uploadSession && stage === "processing"),
    refetchInterval: (query) => {
      const value = query.state.data as AudioRecord | undefined;
      return value && terminalStatuses.has(value.status) ? false : 2_500;
    },
  });

  useEffect(() => {
    const value = statusQuery.data;
    if (!value) return;
    setRecord(value);
    if (value.status === "READY") setStage("ready");
    if (["QUARANTINED", "SCAN_FAILED", "TRANSCODE_FAILED", "DELETED"].includes(value.status)) {
      setError(`Processing stopped with status ${value.status}.`);
    }
  }, [setError, setRecord, setStage, statusQuery.data]);

  const selectFile = (next: File | null) => {
    if (!next) return;
    reset();
    if (!resolveContentType(next)) {
      setError("This audio format is not accepted by the current upload API.");
      return;
    }
    if (next.size <= 0 || next.size > maximumBytes) {
      setError("Select an audio file between 1 byte and 100 MB.");
      return;
    }
    setFile(next);
    setTitle(next.name.replace(/\.[^/.]+$/, ""));
  };

  const onDrop = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    setDragging(false);
    selectFile(event.dataTransfer.files.item(0));
  };

  const runUpload = async (event?: FormEvent) => {
    event?.preventDefault();
    if (!file || !title.trim() || !session) return;
    const controller = new AbortController();
    abortRef.current = controller;
    setError(null);
    try {
      setStage("preparing");
      const nextSession = await createUpload(file, title.trim(), visibility);
      setUploadSession(nextSession);
      setStage("uploading");
      await uploadFile(nextSession, file, ({ transferred: sent, total: size }) => setProgress(sent, size), controller.signal);
      setStage("verifying");
      const completed = await completeUpload(nextSession.audioId);
      setRecord(completed);
      setStage("processing");
    } catch (uploadError) {
      if (uploadError instanceof DOMException && uploadError.name === "AbortError") setError("Upload cancelled. You can select the file and try again.");
      else setError(uploadError instanceof Error ? uploadError.message : "The upload could not be completed.");
    } finally {
      abortRef.current = null;
    }
  };

  const cancel = () => abortRef.current?.abort();
  const progress = total > 0 ? Math.min(100, Math.round((transferred / total) * 100)) : 0;
  const currentStep = stageIndex[stage];

  return (
    <div className="page-stack upload-page">
      <header className="page-title"><p className="eyebrow">PUBLISH A SIGNAL</p><h1>Upload audio without routing it through the app server.</h1><p>Your browser sends the file directly to the quarantine bucket. Processing starts only after the object is verified and scanned.</p></header>

      <ol className="upload-steps" aria-label="Upload progress">
        {["Select file", "Upload", "Process", "Add details", "Publish"].map((label, index) => <li key={label} className={index < currentStep || stage === "ready" ? "is-complete" : index === currentStep ? "is-current" : ""}><span>{index < currentStep || stage === "ready" ? <Check size={15} /> : index + 1}</span><small>{label}</small></li>)}
      </ol>

      {!session ? <div className="notice notice--warning" role="alert"><AlertCircle /><div><strong>{authMode === "disabled" ? "Uploads are temporarily unavailable" : "An authenticated session is required"}</strong><p>{authMode === "disabled" ? <>Public Audio identity is being separated from the internal operator realm. <Link to="/discover">Browse public audio</Link> while sign-in and sign-up are unavailable.</> : <><Link to="/sign-in">Sign in with Keycloak</Link> or configure the development subject locally before uploading.</>}</p></div></div> : null}

      <form className="upload-workspace" onSubmit={runUpload}>
        <div
          className={`drop-zone ${dragging ? "is-dragging" : ""} ${file ? "has-file" : ""}`}
          onDragOver={(event) => { event.preventDefault(); setDragging(true); }}
          onDragLeave={() => setDragging(false)}
          onDrop={onDrop}
        >
          <input ref={inputRef} type="file" accept="audio/mpeg,audio/wav,audio/x-wav,audio/flac,audio/aac,audio/ogg" onChange={(event) => selectFile(event.target.files?.item(0) ?? null)} />
          {file ? <><div className="drop-zone__icon"><FileAudio /></div><div><strong>{file.name}</strong><span>{formatBytes(file.size)} · {file.type}</span></div><button type="button" className="icon-button" onClick={() => { setFile(null); setTitle(""); reset(); }} aria-label="Remove selected file"><X /></button></> : <><div className="drop-zone__icon"><UploadCloud /></div><div><strong>Drop one audio file here</strong><span>MP3, WAV, FLAC, AAC, or OGG · up to 100 MB</span></div><Button type="button" variant="secondary" onClick={() => inputRef.current?.click()}>Choose file</Button></>}
        </div>

        <div className="upload-form-panel">
          <label><span>Track title</span><Input value={title} maxLength={200} onChange={(event) => setTitle(event.target.value)} placeholder="Name this track" disabled={stage !== "idle" && stage !== "failed"} /></label>
          <div className="upload-status" aria-live="polite">
            <div className="upload-status__header"><div><span className="status-dot" /><strong>{stageLabel(stage, record?.status)}</strong></div>{stage === "uploading" ? <span>{progress}%</span> : null}</div>
            <div className="progress-track" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={stage === "uploading" ? progress : stage === "ready" ? 100 : undefined}><span style={{ width: `${stage === "ready" ? 100 : progress}%` }} /></div>
            {total > 0 ? <p>{formatBytes(transferred)} of {formatBytes(total)} transferred</p> : <p>{statusDescription(stage, record?.status)}</p>}
          </div>
          {error ? <div className="inline-error" role="alert"><AlertCircle size={18} /><span>{error}</span></div> : null}
          <div className="upload-form-panel__actions">
            {stage === "uploading" ? <Button variant="secondary" onClick={cancel}>Cancel upload</Button> : null}
            {stage === "failed" ? <Button variant="secondary" onClick={() => { reset(); void runUpload(); }}><RefreshCcw size={16} /> Retry</Button> : null}
            {stage === "ready" && record ? <Link className="button button--primary button--md" to={`/track/${record.id}`}>Open track</Link> : null}
            {stage === "idle" ? <Button type="submit" disabled={!file || !title.trim() || !session}>Prepare upload</Button> : null}
          </div>
        </div>
      </form>

      <section className="metadata-preview"><div><p className="eyebrow">DETAILS AFTER PROCESSING</p><h2>Publishing metadata</h2><p>The current backend accepts the title and visibility during upload. Creator, artwork, genre, mood, tags, and release details will be enabled when the metadata update API is available. Visibility can also be changed later on the track page.</p></div><div className="metadata-fields"><label>Creator<Input disabled placeholder="Identity profile" /></label><label>Genre<Input disabled placeholder="Metadata API required" /></label><label>Visibility<select value={visibility} onChange={(event) => setVisibility(event.target.value as Visibility)} disabled={stage !== "idle"}><option value="private">Private — only you</option><option value="public">Public — listed in Discover</option></select></label><label>Description<textarea disabled placeholder="Metadata API required" /></label></div></section>
    </div>
  );
}

function stageLabel(stage: UploadStage, status?: string) {
  if (stage === "preparing") return "Calculating checksum and preparing upload";
  if (stage === "uploading") return "Uploading directly to storage";
  if (stage === "verifying") return "Verifying the stored object";
  if (stage === "processing") return status ? `Processing · ${status}` : "Waiting for scan and processing";
  if (stage === "ready") return "Waveform and playback artifact ready";
  if (stage === "failed") return "Upload or processing failed";
  return "Ready to prepare upload";
}

function statusDescription(stage: UploadStage, status?: string) {
  if (stage === "processing") return status ? `Current backend status: ${status}` : "Preparing your track for playback.";
  if (stage === "ready") return "Your track has completed processing.";
  return "Real progress will appear after the upload starts.";
}
