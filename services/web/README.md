# audio-web

Cantaloupe 오디오 서비스의 React·TypeScript 프론트엔드다. `services/api`가 현재
제공하는 업로드 생성, 업로드 완료, 오디오 단건 조회와 재생 접근 API에 직접 연결한다.

## 구현 경계

- `/`: 시네마틱 영상 배경의 단일 화면 랜딩
- `/discover`, `/search`, `/library`: 실제 목록 API를 기다리는 empty/error 상태
- `/upload`: SHA-256 계산, Presigned PUT, 실제 전송률, 완료 검증, 처리 상태 polling
- `/track/:trackId`: 실제 오디오 상태, 만료 재생 URL과 waveform JSON 조회
- 전역 Player: 단일 HTML audio, Zustand 상태와 사전 생성 peak 기반 Wavesurfer

Mock adapter, 가짜 트랙, 가짜 통계, placeholder audio는 포함하지 않는다. 목록·검색,
Creator, Playlist API가 없는 화면은 데이터를 만들어내지 않고 빈 상태를 표시한다.

## 로컬 실행

백엔드와 LocalStack을 먼저 실행한다.

```bash
cd ../..
make local-up
```

다른 터미널에서 Web을 실행한다.

```bash
cd services/web
cp .env.example .env.local
npm install
npm run dev
```

Vite 개발 서버는 `/v1`을 `http://localhost:8080`으로 전달한다. Presigned URL의
S3 PUT은 브라우저에서 LocalStack으로 직접 전송하며, 로컬 버킷 CORS는
`scripts/localstack-init.sh`가 설정한다.

## 검증

```bash
npm run typecheck
npm run build
```

## 공개 설정

`VITE_*` 값은 빌드 결과에 포함되는 공개 값이다. AWS Access Key, Secret Key,
Cognito client secret 같은 비밀 값은 넣지 않는다. `VITE_AUTH_MODE=development`와
`VITE_DEV_SUBJECT`는 로컬 API 검증에만 사용한다.
