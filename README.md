# app-audio

오디오 서비스 소스. 세 서비스를 한 리포에 둔다.

```
services/api/      업로드·메타데이터·재생 URL API
services/worker/   ffmpeg 트랜스코딩
services/web/      프론트엔드
shared/schema/     api 와 worker 가 주고받는 메시지 형식
```

## 왜 한 리포인가

`shared/schema/` 때문이다. API 가 큐에 넣는 메시지를 워커가 읽는데, 리포를 나누면
스키마 변경이 두 PR 로 쪼개진다. 그 사이에 배포 순서가 어긋나면 워커가 메시지를
못 읽는다.

이미지는 서비스마다 따로 빌드하므로 배포는 여전히 독립적이다.

## 아키텍처

오디오 업로드, 악성코드 검사 경계, SQS, FFmpeg 변환, MP3 재생, waveform,
정적 Worker 기준과 자동 확장 도입 판단 기준은
[`docs/audio-service-architecture.md`](docs/audio-service-architecture.md)에
정리한다.

## 빌드 시 흐름

`main` 푸시마다 GitHub Actions가 세 서비스를 빌드해 Tailscale HTTPS Harbor로
푸시한다. `latest`와 배포 추적용 커밋 SHA 태그를 함께 만든다.

```
푸시 → GitHub Actions → Harbor
     → 02-k8s-manifests의 이미지 SHA 갱신
     → Argo CD 동기화
```

이미지 경로는 다음과 같다.

```
cntlp-onp-wk-01.tail270b85.ts.net/library/app-audio-api:<git-sha>
cntlp-onp-wk-01.tail270b85.ts.net/library/web:<git-sha>
cntlp-onp-wk-01.tail270b85.ts.net/library/worker:<git-sha>
```

Harbor의 `library` 프로젝트는 공개 Pull이므로 Kubernetes에는 Registry 자격증명을
배포하지 않는다. Push 자격증명은 GitHub Actions Secret으로만 관리한다.

GitOps 갱신 단계에는 `02-k8s-manifests` Contents 쓰기 권한만 가진
`MANIFESTS_TOKEN` Secret이 필요하다. 이 값이 없으면 Harbor 이미지 Push까지는
완료되지만 매니페스트 저장소 체크아웃 단계에서 중단된다.

수동 빌드는 동일한 Harbor 경로를 사용한다.

```bash
docker buildx build --platform linux/amd64 \
  -t cntlp-onp-wk-01.tail270b85.ts.net/library/app-audio-api:<git-sha> \
  --push services/api

docker buildx build --platform linux/amd64 \
  --build-arg VITE_DEV_SUBJECT=browser-tester \
  -t cntlp-onp-wk-01.tail270b85.ts.net/library/web:<git-sha> \
  --push services/web
```

매니페스트는 가변 `dev` 대신 커밋 SHA 태그를 사용한다. Argo CD는
`02-k8s-manifests`의 태그 변경을 감지해 새 이미지를 배포한다.

### Web 공개 설정

`VITE_API_BASE_URL`은 빌드 시점에 번들로 굳힌다. 비우면 Web이 같은 출처의 API를
호출한다.

인증 설정은 `/config/runtime-config.js`에서 읽는다. 운영 환경에서는 Kubernetes
ConfigMap을 이 경로에 마운트하므로 같은 Web 이미지를 다시 빌드하지 않고 API와
같은 Argo CD 동기화에서 인증 모드를 전환할 수 있다. 이미지에 포함된 빈 설정 파일은
독립 실행 시 안전한 기본값이며, 로컬 개발에서는 `VITE_*` 값을 fallback으로 쓴다.

```
VITE_AUTH_MODE     로컬 fallback. 기본 development
VITE_DEV_SUBJECT   로컬 개발용 subject
VITE_API_BASE_URL  비우면 같은 출처로 호출
```

운영 Keycloak OIDC 설정은 `02-k8s-manifests/apps/audio/web/runtime-config.yaml`에서
관리한다.

```text
authMode: oidc
oidcIssuerUrl: https://<keycloak-host>/realms/<realm>
oidcClientId: <public-spa-client>
oidcRedirectUri: https://<audio-host>/auth/callback
oidcPostLogoutRedirectUri: https://<audio-host>/
oidcScope: openid profile email
```

SPA Client에는 client secret을 만들거나 주입하지 않는다. Web은 Authorization
Code + PKCE로 받은 Access Token을 `Authorization: Bearer` 헤더에 넣고, API는
`OIDC_ISSUER_URL`의 JWKS와 `OIDC_AUDIENCE`로 서명·발급자·대상·만료·`sub`를 검증한다.

API는 런타임에 다음 값을 받는다.

```text
AUTH_MODE=oidc
OIDC_ISSUER_URL=https://<keycloak-host>/realms/<realm>
OIDC_AUDIENCE=<api-audience>
```

Keycloak의 Web Client에는 `<api-audience>`를 Access Token의 `aud`에 넣는 Audience
Mapper 또는 동등한 Client Scope가 필요하다. 그렇지 않으면 로그인에는 성공해도
API가 다른 서비스용 토큰으로 판단해 `401`을 반환한다.

## API

```
POST   /v1/audios/uploads          업로드 세션 생성. Presigned PUT URL 발급
                                   title, content_type, content_length,
                                   checksum_sha256, visibility(선택, 기본 private)
POST   /v1/audios/{id}/complete    업로드 확정. 크기·타입·버전 검증 후 스캔 제출
GET    /v1/audios                  본인 트랙 목록. 상태 무관, 최신순
GET    /v1/audios?scope=public     공개 카탈로그. 소유자 무관, public + READY만
GET    /v1/audios/{id}             상세. 비공개는 소유자만
PATCH  /v1/audios/{id}             공개 여부 전환. 소유자만
GET    /v1/audios/{id}/playback    CloudFront Signed URL. READY만, 3시간 만료
```

목록은 커서 기반이다. `next_cursor`가 없으면 마지막 페이지다. OFFSET을 쓰지
않는 이유는 페이지를 넘기는 도중 새 업로드가 들어오면 행이 밀려 중복과 누락이
생기기 때문이다.

**공개는 목록 노출과 상세 조회 허용을 뜻한다.** 공개해도 재생 URL은 여전히
서명이 필요하고 만료된다.

## 마이그레이션

`services/api/migrations/`에 순서대로 둔다. 실행기가 없으므로 배포 전에 직접
적용한다.

```
001_init.sql             초기 스키마
002_public_catalog.sql   공개 카탈로그용 부분 인덱스
```

## 현재 구현 범위

첫 번째 MVP 기반은 다음 경계까지 구현한다.

- `audio-api`: 업로드 요청 검증, 불변 S3 key 생성, Presigned PUT, HEAD 검증과 개발용 Scan Adapter
- `audio-events`: Scan 결과와 Worker 결과의 중복 처리, Transactional Outbox
- `audio-transcode`: CLEAN tag와 checksum 재검증, MP3와 waveform 생성, 결과 발행
- `audio-web`: 단일 시네마틱 랜딩, 실제 Presigned 업로드, 처리 상태 polling, MP3 재생과 사전 생성 waveform 표시
- `audio-api`: READY·소유권 확인 후 MP3와 waveform 만료 URL 발급
- 로컬 S3 Presigned GET과 운영 CloudFront SHA-256 Signed URL을 같은 서명 경계로 분리
- CloudFront signing private key를 Kubernetes Secret으로 mount하고 API 환경 변수와 연결
- PostgreSQL 초기 스키마와 SQS 메시지 계약
- PostgreSQL·LocalStack을 사용하는 로컬 실행 구성
- Kubernetes 배포 매니페스트 (`02-k8s-manifests/apps/audio`)
- 본인 트랙 목록과 공개 카탈로그, 공개 여부 전환
- Keycloak과 호환되는 OIDC Authorization Code + PKCE Web 흐름과 API JWT 검증

아직 실제 환경에서 검증하지 않은 경계는 다음과 같다.

- Keycloak Realm·Client 연결과 로그인·소유권 분리 E2E. 현재 배포는
  `AUTH_MODE=development`를 유지한다.
- 검색·Creator·Playlist API와 해당 화면의 실제 데이터
- 실제 악성코드 검사기

`01-infra-provisioning`의 S3, SQS와 CloudFront는 적용됐다.

## AWS 데이터 경로 E2E

2026-08-06에 `https://audio.echoprism.cloud`로 API와 브라우저 양쪽에서 전
구간을 통과시켰다. 브라우저에서만 드러난 결함이 셋 있었다.

```
VITE_DEV_SUBJECT 미전달   업로드 화면 자체가 차단
S3 CORS Origin 누락        파일 선택 후 전송에서 차단
라이브러리 기본 탭         데이터 없는 탭이 먼저 열려 목록이 비어 보임
```

curl은 헤더를 직접 붙이고 Origin 검사도 받지 않는다. **API 레벨 검증만으로는
이 셋 중 어느 것도 발견되지 않는다.**

```
업로드 URL 발급 → S3 PUT → complete → 스캔 → 변환 → 재생 URL → CloudFront 수신
```

산출물은 `targets` 지시값과 일치했다.

```
playback.mp3    MPEG layer III, 192 kbps, 44.1 kHz   (mp3_bitrate_kbps=192)
waveform.json   duration_ms=3000, points_per_second=20
서명 없는 접근    403
```

## 업로드 무결성 검증 위치

**SHA-256 대조는 `audio-transcode`가 한다.** `/complete`가 아니다.

Presigned URL에 체크섬을 넣으면 SigV4 서명 과정에서 `x-amz-checksum-sha256`이
쿼리스트링으로 옮겨진다. 그러면 두 경로 모두 막힌다.

```
헤더로 보냄   S3가 403. 서명되지 않은 x-amz-* 헤더는 거부된다
헤더를 뺌     업로드는 되지만 S3가 SHA-256을 기록하지 않는다
```

그래서 각 단계가 확인할 수 있는 것만 확인한다.

```
/complete           크기, Content-Type, S3 Version
audio-transcode     내려받은 바이트로 SHA-256 계산 후 대조
                    불일치 시 SOURCE_CHECKSUM_MISMATCH (재시도 안 함)
```

워커는 FFmpeg를 위해 원본 전체를 이미 내려받으므로 추가 전송 비용이 없다.
큰 파일에서 메모리를 쓰지 않도록 1 MiB 단위로 스트리밍해 해시한다.

클라이언트는 지금도 업로드 요청에 `checksum_sha256`을 담아 보낸다. 그 값이
DB에 저장돼 변환 작업 메시지로 전달되고, 워커가 그것과 대조한다.

현재 악성코드 검사기는 없다. `audio-api`의 개발용 Scan Adapter는 `/complete`에서
정확한 S3 Version을 검증한 뒤 `CntlpScanStatus=NO_THREATS_FOUND` 태그와
`scan-result` 메시지를 기록한다. 이는 Queue·상태 기계·Worker 계약을 검증하기
위한 대체 경로이지 실제 보안 검사가 아니다. OIDC 인증은 이 검사 경계를
대체하지 않으므로 실제 검사기 도입 전에는 공개 업로드 서비스로 완료 판정하지 않는다.

개발용 인증은 공개 환경에 배포하지 않는다.

## 로컬 검증

Worker 도구를 처음 준비할 때 다음 명령을 실행한다.

```bash
cd services/worker
python3 -m venv .venv
.venv/bin/pip install '.[dev]'
```

단위 테스트와 정적 검사는 저장소 루트에서 실행한다.

```bash
make validate
```

로컬 서비스는 Docker Compose로 실행한다.

```bash
make local-up

# 실제 업로드부터 변환·재생 URL·waveform까지 검증
make local-e2e

curl http://localhost:8080/healthz
curl http://localhost:8081/healthz
curl http://localhost:9090/metrics
```

Web은 별도 터미널에서 실행한다. Mock 데이터는 사용하지 않는다.

```bash
cd services/web
npm install
npm run dev
```

로컬 구성의 S3와 SQS는 AWS 자원을 만들지 않는다. `AUTH_MODE=development`,
LocalStack용 더미 AWS 자격 증명과 PostgreSQL의 로컬 trust 인증은 Compose 밖에서
사용하지 않는다.

## 스키마를 고칠 때

`shared/schema/` 를 바꾸면 api 와 worker 를 **같은 커밋에서** 맞춰야 한다.
한쪽만 배포되면 메시지를 못 읽는다.
