import { useAuthStore } from "../stores/authStore";
import { usePlayerStore } from "../stores/playerStore";

export function SettingsPage() {
  const session = useAuthStore((state) => state.session);
  const volume = usePlayerStore((state) => state.volume);
  const setVolume = usePlayerStore((state) => state.setVolume);
  return <div className="page-stack"><header className="page-title"><p className="eyebrow">SETTINGS</p><h1>Playback and account.</h1><p>Only device-local playback preferences are editable before the account API is connected.</p></header><section className="settings-panel"><div><h2>Session</h2><p>{session ? `Using ${session.mode} identity: ${session.subject}` : "No identity provider session is available."}</p></div><div><label htmlFor="settings-volume"><span><strong>Default volume</strong><small>Stored for the current browser session.</small></span><output>{Math.round(volume * 100)}%</output></label><input id="settings-volume" type="range" min="0" max="1" step="0.01" value={volume} onChange={(event) => setVolume(Number(event.target.value))} /></div><div><h2>Authentication</h2><p>Cognito region, user pool, and client ID are public runtime settings. Tokens and AWS credentials are never stored in source code.</p></div></section></div>;
}
