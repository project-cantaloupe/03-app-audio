export type AuthSession = {
  subject: string;
  mode: "development" | "cognito";
};

export function getSession(): AuthSession | null {
  const mode = import.meta.env.VITE_AUTH_MODE ?? "development";
  if (mode === "development") {
    const subject = import.meta.env.VITE_DEV_SUBJECT?.trim();
    return subject ? { subject, mode } : null;
  }

  // Cognito 토큰 검증은 백엔드와 인증 구성이 준비된 뒤 이 경계에 연결한다.
  return null;
}

export function getAuthHeaders(): HeadersInit {
  const session = getSession();
  if (!session) return {};
  if (session.mode === "development") {
    return { "X-Cantaloupe-Subject": session.subject };
  }
  return {};
}
