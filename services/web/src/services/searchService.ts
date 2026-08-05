export type SearchCapability = {
  available: false;
  reason: string;
};

// 검색 API 계약이 아직 없으므로 가짜 결과를 반환하지 않고 기능 상태만 제공한다.
export function getSearchCapability(): SearchCapability {
  return { available: false, reason: "Search will be enabled when the track index API is available." };
}
