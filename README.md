# app-audio

오디오 서비스 소스. 세 서비스를 한 리포에 둔다.

```
services/api/      업로드 접수, 스트리밍 API
services/worker/   ffmpeg 트랜스코딩
services/web/      프론트엔드
shared/schema/     api 와 worker 가 주고받는 메시지 형식
```

## 왜 한 리포인가

`shared/schema/` 때문이다. API 가 큐에 넣는 메시지를 워커가 읽는데, 리포를 나누면
스키마 변경이 두 PR 로 쪼개진다. 그 사이에 배포 순서가 어긋나면 워커가 메시지를
못 읽는다.

이미지는 서비스마다 따로 빌드하므로 배포는 여전히 독립적이다.

## 빌드하면 무슨 일이 일어나나

```
푸시 → Jenkins 빌드 → 이미지 푸시 → k8s-manifests 에 PR → auto-merge → ArgoCD 배포
```

**Jenkins 는 매니페스트에 직접 푸시하지 않고 PR 을 연다.** 직접 푸시하려면 브랜치
보호를 우회해야 하는데, 우회 권한은 경로를 가리지 않는다. 그러면 봇이
`governance/` 도 고칠 수 있게 된다.

## 스키마를 고칠 때

`shared/schema/` 를 바꾸면 api 와 worker 를 **같은 커밋에서** 맞춰야 한다.
한쪽만 배포되면 메시지를 못 읽는다.
