import { useEffect, useRef } from "react";

const backgroundVideo =
  "https://d8j0ntlcm91z4.cloudfront.net/user_38xzZboKViGWJOttwIXH07lWA1P/hf_20260406_094145_4a271a6c-3869-4f1c-8aa7-aeb0cb227994.mp4";

type CinematicVideoBackgroundProps = {
  reducedMotion: boolean;
  variant?: "landing" | "app";
};

export function CinematicVideoBackground({ reducedMotion, variant = "landing" }: CinematicVideoBackgroundProps) {
  const videoRef = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;
    if (reducedMotion) {
      video.pause();
      return;
    }
    void video.play().catch(() => {
      // 자동 재생이 차단돼도 텍스트와 탐색 기능은 그대로 유지한다.
    });
  }, [reducedMotion]);

  return (
    <>
      <video
        ref={videoRef}
        className={variant === "app" ? "app-background-video" : "landing-video"}
        src={backgroundVideo}
        autoPlay={!reducedMotion}
        muted
        loop
        playsInline
        preload="metadata"
        aria-hidden="true"
        tabIndex={-1}
      />
      <div className={variant === "app" ? "app-background-blur" : "landing-bottom-blur"} aria-hidden="true" />
    </>
  );
}
