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

```
푸시 → Jenkins 빌드 → 이미지 푸시 → k8s-manifests 에 PR → auto-merge → ArgoCD 배포
```

**Jenkins 는 매니페스트에 직접 푸시하지 않고 PR 을 연다.** 직접 푸시하려면 브랜치
보호를 우회해야 하는데, 우회 권한은 경로를 가리지 않는다. 그러면 봇이
`governance/` 도 고칠 수 있게 된다.

Jenkins agent의 `onp-devops`는 Kubernetes `area` 라벨이 아니라 Jenkins가
관리하는 실행기 라벨이다. 해당 실행기는 On-prem DevOps 환경에 둔다.

각 서비스의 Dockerfile과 `02-k8s-manifests/apps/audio/{api,worker,web}` 이미지
Kustomization이 함께 준비된 뒤 파이프라인을 활성화한다. 현재 파이프라인은 아직
활성화하지 않는다.

## 현재 구현 범위

첫 번째 MVP 기반은 다음 경계까지 구현한다.

- `audio-api`: 업로드 요청 검증, 불변 S3 key 생성, Presigned PUT, HEAD 검증과 개발용 Scan Adapter
- `audio-events`: Scan 결과와 Worker 결과의 중복 처리, Transactional Outbox
- `audio-transcode`: CLEAN tag와 checksum 재검증, MP3와 waveform 생성, 결과 발행
- `audio-web`: 단일 시네마틱 랜딩, 실제 Presigned 업로드, 처리 상태 polling, MP3 재생과 사전 생성 waveform 표시
- `audio-api`: READY·소유권 확인 후 MP3와 waveform 만료 URL 발급
- 로컬 S3 Presigned GET과 운영 CloudFront SHA-256 Signed URL을 같은 서명 경계로 분리
- PostgreSQL 초기 스키마와 SQS 메시지 계약
- PostgreSQL·LocalStack을 사용하는 로컬 실행 구성

아직 구현하지 않은 경계는 다음과 같다.

- Cognito JWT 검증. 현재 `AUTH_MODE=development`에서만 개발용 subject header 사용
- CloudFront signing private key의 Kubernetes Secret mount와 API 환경 변수 연결
- 목록·검색·Creator·Playlist API와 해당 화면의 실제 데이터
- 실제 악성코드 검사기와 AWS Audio 데이터 경로 E2E 검증
- Kubernetes 배포 매니페스트

`01-infra-provisioning`의 S3, SQS와 CloudFront는 적용됐다. 이 상태는 AWS Resource
준비를 의미하며 Application의 실제 AWS 통합 검증을 의미하지 않는다.

현재 악성코드 검사기는 없다. `audio-api`의 개발용 Scan Adapter는 `/complete`에서
정확한 S3 Version을 검증한 뒤 `CntlpScanStatus=NO_THREATS_FOUND` 태그와
`scan-result` 메시지를 기록한다. 이는 Queue·상태 기계·Worker 계약을 검증하기
위한 대체 경로이지 실제 보안 검사가 아니다. API는 현재 `AUTH_MODE=development`
외에는 시작하지 않으므로 이 경로를 공개 환경에 배포하지 않는다.

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

# CI/CD pipeline test
