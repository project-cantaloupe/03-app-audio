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

오디오 업로드, GuardDuty 검사, SQS, FFmpeg 변환, MP3 재생, waveform,
KEDA·AWS Worker 자동 확장 구조와 선택 근거는
[`docs/audio-service-architecture.md`](docs/audio-service-architecture.md)에
정리한다.

## 빌드하면 무슨 일이 일어나나

```
푸시 → Jenkins 빌드 → 이미지 푸시 → k8s-manifests 에 PR → auto-merge → ArgoCD 배포
```

**Jenkins 는 매니페스트에 직접 푸시하지 않고 PR 을 연다.** 직접 푸시하려면 브랜치
보호를 우회해야 하는데, 우회 권한은 경로를 가리지 않는다. 그러면 봇이
`governance/` 도 고칠 수 있게 된다.

Jenkins agent의 `onp-devops`는 Kubernetes `area` 라벨이 아니라 Jenkins가
관리하는 실행기 라벨이다. 해당 실행기는 On-prem DevOps 환경에 둔다.

현재 `services/*/src`는 서비스 구현을 넣기 위한 골격이다. 각 서비스의
Dockerfile과 `02-k8s-manifests/apps/audio/{api,worker,web}` 이미지
Kustomization이 함께 준비된 뒤 파이프라인을 활성화한다.

## 스키마를 고칠 때

`shared/schema/` 를 바꾸면 api 와 worker 를 **같은 커밋에서** 맞춰야 한다.
한쪽만 배포되면 메시지를 못 읽는다.
