-- 공개 카탈로그 조회용 인덱스다.
--
-- ListPublicAudios 의 WHERE 절과 ORDER BY 를 그대로 담는다. 부분 인덱스라
-- 비공개나 처리 중인 행은 인덱스에 들어가지 않는다. 전체 트랙 중 공개된
-- 것이 일부일 때 크기가 작게 유지된다.
--
-- 기존 audios_owner_created_idx 는 owner_subject 가 선행 컬럼이라 소유자를
-- 가리지 않는 이 질의에는 쓰이지 않는다.
CREATE INDEX IF NOT EXISTS audios_public_created_idx
    ON audios (created_at DESC, id DESC)
    WHERE deleted_at IS NULL
      AND visibility = 'public'
      AND status = 'READY';
