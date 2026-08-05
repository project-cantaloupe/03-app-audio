# Audio Service Architecture

## 1. 문서 상태

- 상태: MVP 구현 기준
- 변경일: 2026-08-05
- 외부 진입 결정: ALB 직접 라우팅 대신 NLB + Istio Ingress Gateway
- 구현 상태: 로컬 업로드→개발용 Scan Adapter→변환→Signed URL→재생 artifact E2E 검증,
  AWS Audio Terraform 적용 완료, 실제 악성코드 검사기 미채택, Kubernetes
  Application Runtime 미적용
- 대상: Cantaloupe 오디오 업로드·검사·변환·재생 서비스
- 애플리케이션 저장소: `03-app-audio`
- Kubernetes 매니페스트 저장소: `02-k8s-manifests`
- 인프라 저장소: `01-infra-provisioning`

이 문서는 `03-app-audio` 구현 코드보다 먼저 서비스 경계, 데이터 흐름, 실패 처리와 선택 근거를
고정한다. 세부 라이브러리 버전과 조정 가능한 용량 값은 각 구현 저장소에서
관리한다.

## 2. 목표와 비목표

### 목표

- 사용자가 웹에서 오디오를 업로드하고 처리 상태를 확인한다.
- 악성코드 검사를 통과한 파일만 FFmpeg 처리와 재생에 사용한다.
- 브라우저가 MP3를 seek하며 재생하고, 미리 생성된 파형 위에 현재 위치를 표시한다.
- DB 상태 변경과 Queue 메시지 발행 사이의 유실을 방지한다.
- 중복 메시지와 Worker 재시작에도 같은 작업을 안전하게 재실행한다.
- SQS 적체량과 Worker 처리량을 측정해 자동 확장 도입 여부를 판단한다.
- 부하와 비용을 함께 측정할 수 있는 관측 지점을 둔다.
- Internet-facing NLB와 Istio Ingress Gateway로 공개 HTTPS 진입점을 구성한다.
- Gateway에서 stable 단일 route, mTLS, 접근 정책과 HTTP 관측을 검증한다.

### MVP 비목표

- 라이브 방송과 실시간 녹음
- HLS Adaptive Bitrate Streaming
- 댓글, 좋아요, 팔로우, 추천 피드
- DRM과 광고 삽입
- GCP·On-Prem Node에서 사용자 오디오 처리
- GCP·On-Prem Workload까지 확장하는 멀티클러스터 Service Mesh
- S3·SQS·RDS 같은 AWS 관리형 서비스의 Istio Mesh 편입
- Canary 트래픽 분배와 자동 승격

HLS는 MP3 기준선의 재생 품질과 비용을 측정한 뒤 비교 실험으로 추가한다.

## 3. 확정한 선택

| 영역 | 선택 | 이유 |
| --- | --- | --- |
| 개발 범위 | 핵심 오디오 MVP | 비동기 처리와 장애 복구를 먼저 검증한다. |
| API | Go | 작은 메모리 사용량과 명확한 동시성·타임아웃 제어를 우선한다. |
| Worker | Python + FFmpeg | 미디어 처리 도구 연동과 실험 속도를 우선한다. |
| Queue | SQS Standard + DLQ | Queue 서버를 클러스터에서 운영하지 않고 at-least-once 전달을 멱등 처리로 흡수한다. |
| 악성코드 검사 | 미채택. 개발용 Scan Adapter가 계약만 유지 | 대상 AWS 계정이 Free account plan이라 GuardDuty를 구독할 수 없고, ClamAV는 단일 Worker의 메모리 기준선을 깬다. 격리·계약·상태 기계는 유지하고 검사기 자리만 비워 둔다. |
| 재생 | Progressive MP3 | VOD 오디오에 HLS playlist·segment 복잡도를 추가하지 않고 Range GET으로 seek한다. |
| 파형 | 사전 생성한 JSON | 재생 시 원본을 다시 분석하지 않고 브라우저에는 작은 peak 데이터만 전달한다. |
| 인증 | Cognito | 비밀번호를 애플리케이션 DB에서 관리하지 않고 JWT subject를 소유자 식별자로 쓴다. |
| 외부 진입 | Internet-facing NLB + Istio Ingress Gateway | NLB는 L4 연결만 전달하고 TLS와 HTTP 라우팅은 Istio가 소유한다. |
| TLS | Istio Gateway 종료 + cert-manager DNS-01 | 공개 인증서를 자동 갱신하고 private key를 Git 밖의 Kubernetes Secret으로 관리한다. |
| Service Mesh 범위 | `audio-web`, `audio-api` | 실제 HTTP 진입 경로에만 Sidecar를 적용하고 비동기 Worker 비용과 장애 범위를 분리한다. |
| 업로드 | 최대 100 MB 단일 Presigned PUT | Multipart 수명주기보다 MVP 단순성을 우선한다. |
| Worker 실행 | AWS Service Node의 정적 Deployment 1개 | 먼저 기능·장애 복구·자원 사용량을 검증한다. |
| Kubernetes 범위 | `apps` Namespace + `audio-*` Workload | Namespace는 운영 영역, 이름과 라벨은 오디오 애플리케이션을 구분한다. |

KEDA와 AWS Worker ASG는 확정한 MVP 구성요소가 아니다. 정적 Worker의 Queue
대기시간, 유휴 비용과 처리량을 측정한 뒤 필요할 때만 별도 승인으로 도입한다.

## 4. 배치 원칙

- Kubernetes 클러스터는 하나다.
- 사용자 서비스는 AWS에서만 실행한다.
- GCP·On-Prem Node는 FinOps·DevOps·관측 용도이며 오디오 요청을 처리하지 않는다.
- 애플리케이션 Workload 이름에는 `cntlp`나 `aws`를 넣지 않는다.
- 실행 위치는 Node selector로 결정한다.
- Istio Ingress Gateway와 Mesh 대상 Workload는 AWS Service Node에만 배치한다.
- `audio-events`와 `audio-transcode`에는 Sidecar를 주입하지 않는다.

사용자 서비스는 `apps` Namespace에 배포한다.
`02-k8s-manifests/apps/audio/`는 Git 저장소의 영역·애플리케이션 디렉터리이며,
`audio`라는 별도 Namespace를 뜻하지 않는다. 오디오 도메인은 `audio-*` Workload
이름과 `app` 라벨로 구분한다.

```yaml
metadata:
  namespace: apps
  labels:
    app: audio-api
    area: apps
    platform: aws
spec:
  nodeSelector:
    platform: aws
    role: service
```

| Workload | 형태 | 책임 |
| --- | --- | --- |
| `audio-web` | Deployment | 업로드·상태·재생 UI |
| `audio-api` | Deployment | HTTP API, Cognito JWT 검증, Presigned URL과 재생 URL 발급 |
| `audio-events` | Deployment | Scan 결과·Worker 결과 소비, Outbox 발행 |
| `audio-transcode` | Deployment, replica 1 | SQS를 long polling하고 메시지를 한 건씩 처리 |

`istio-ingress`는 전용 `istio-ingress` Namespace의 별도 Deployment와 NodePort
Service로 운영한다. Control Plane에는 Gateway와 사용자 Workload를 배치하지 않는다.

## 5. 전체 구조

```mermaid
flowchart LR
    Client["Web Client"]
    Cognito["Amazon Cognito"]
    DNS["Public DNS"]
    NLB["AWS NLB<br/>TCP 80/443"]
    CloudFront["CloudFront"]

    subgraph Cluster["cntlp-k8s"]
        Ingress["Istio Ingress Gateway"]
        Web["audio-web"]
        API["audio-api"]
        Events["audio-events"]
        Worker["audio-transcode"]
    end

    subgraph Data["AWS Data Plane"]
        RDS["PostgreSQL RDS"]
        Incoming["S3 quarantine bucket"]
        Artifacts["S3 audio artifact bucket"]
        ScanQueue["SQS scan-result"]
        JobQueue["SQS transcode"]
        ResultQueue["SQS transcode-result"]
        DLQ["SQS DLQs"]
    end

    Client --> Cognito
    Client --> DNS
    DNS --> NLB
    NLB --> Ingress
    Ingress --> Web
    Ingress --> API
    API --> RDS
    API -. "Presigned PUT" .-> Client
    Client --> Incoming

    API -. "HEAD 검증 + 개발용 스캔 상태 태그" .-> Incoming
    API -. "scan-result 발행" .-> ScanQueue
    ScanQueue --> Events
    Events --> RDS
    Events --> JobQueue

    JobQueue --> Worker
    Worker --> Incoming
    Worker --> Artifacts
    Worker --> ResultQueue
    ResultQueue --> Events

    JobQueue -. "failed messages" .-> DLQ
    ResultQueue -. "failed messages" .-> DLQ

    API -. "Signed playback URL" .-> Client
    Client --> CloudFront
    CloudFront --> Artifacts
```

CloudFront는 S3 Origin Access Control로 artifact bucket만 읽는다. S3의 직접
Public Access는 차단한다.

### 외부 진입과 TLS 경계

NLB는 HTTP 내용을 해석하지 않는 L4 진입점으로만 사용한다. `443/TCP` 연결은
TLS를 종료하지 않고 Istio Ingress Gateway의 고정 NodePort로 전달한다. 공개
인증서 종료, SNI와 HTTP 경로 라우팅은 Gateway가 담당한다.

```text
Public DNS
  -> Internet-facing NLB TCP:443
  -> AWS Service Node TCP:30443
  -> istio-ingress HTTPS
  -> audio-web 또는 audio-api
```

HTTP는 별도 NLB listener를 통해 Gateway로 전달한 뒤 Istio에서 HTTPS로
리다이렉트한다.

```text
NLB TCP:80  -> NodePort 30080 -> Istio HTTPS redirect
NLB TCP:443 -> NodePort 30443 -> Istio TLS termination
NLB health  -> NodePort 32021 -> /healthz/ready
```

NodePort 값은 Terraform Target Group과 Kubernetes Service 사이의 계약이므로
변경 시 `01-infra-provisioning`과 `02-k8s-manifests`를 같은 변경 단위로 검토한다.
NLB는 AWS Worker EC2 instance ID를 target으로 등록한다. Gateway Service는
`externalTrafficPolicy: Local`을 사용하고, NLB Target Group에는 Gateway Pod가
실제로 배치된 AWS Service Node만 등록한다.

MVP는 AWS Service Node가 있는 가용 영역 하나만 NLB에 활성화한다. instance
target의 client IP preservation과 `externalTrafficPolicy: Local`을 함께 사용해
Gateway access log에서 원본 client IP를 확인한다. 두 번째 AWS Service Node를
추가할 때만 두 번째 Public Subnet·NLB 가용 영역·Gateway replica를 함께 추가한다.

NLB Security Group은 인터넷의 `80/443`만 허용한다. AWS Worker Security Group은
NLB Security Group에서 들어오는 `30080`, `30443`, `32021`만 허용하고 NodePort의
직접 인터넷 접근을 금지한다. Control Plane과 GCP·On-Prem Node는 Target Group에
등록하지 않는다.

NLB health check는 Gateway 준비 상태를 감지하기 위한 신호이며 접근 제어 수단이
아니다. 모든 Target이 동시에 비정상이면 NLB가 fail-open으로 Target에 연결을
시도할 수 있으므로 Gateway의 readiness, 외부 synthetic check와 경보를 함께 둔다.

MVP의 AWS Service Node가 하나이므로 NLB를 사용해도 애플리케이션이 고가용성이
되는 것은 아니다. 단일 Target 또는 Gateway가 실패하면 서비스가 중단된다. 두
AZ 고가용성은 AWS Service Node와 Gateway replica를 AZ별로 확보한 뒤 별도
승인으로 확장한다.

AWS Worker를 재생성하면 instance ID가 바뀌므로 NLB Target 등록도 Terraform으로
갱신해야 한다. 적용 순서는 Network → Compute → Edge, 삭제 순서는 Edge → Compute
→ Network로 유지하고 Target을 콘솔에서 수동 등록하지 않는다.

### 공개 인증서

공개 DNS 이름은 변경 가능한 배포 값으로 관리하고 문서에 실제 도메인을
하드코딩하지 않는다. cert-manager는 DNS-01 방식으로 인증서를 발급하고
`istio-ingress`의 TLS Secret을 갱신한다. DNS provider가 Route 53이면 cert-manager
ServiceAccount에는 지정 Hosted Zone의 DNS 검증 레코드에 필요한 최소 IAM 권한만
부여한다.

인증서 private key와 발급 계정 credential은 Git, Terraform 변수, Queue와 로그에
넣지 않는다. 인증서가 준비되지 않으면 NLB Target이 정상이어도 공개 HTTPS 완료로
판정하지 않는다.

### Istio 라우팅

MVP는 `networking.istio.io/v1`의 Gateway, VirtualService와 DestinationRule을 한
세트로 사용한다. 같은 host의 기본 route에 Kubernetes Gateway API를 혼합하지
않는다. 기본 경로는 다음과 같다.

| 요청 | 대상 | 비고 |
| --- | --- | --- |
| `/v1/*` | `audio-api` | Cognito JWT 검증은 API가 계속 수행 |
| `/` | `audio-web` | 정적 UI와 클라이언트 라우팅 |
| `/metrics` | 외부 라우팅 없음 | Prometheus의 클러스터 내부 scrape만 허용 |
| `/healthz`, `/readyz` | 외부 라우팅 없음 | Kubernetes probe와 내부 검증에만 사용 |

Gateway `AuthorizationPolicy`에서 `/metrics`, `/healthz`, `/readyz` 외부 요청을
먼저 거부한 뒤 `/v1/*`, `/` 순서로 route를 평가한다. `/`의 Web fallback이 운영
endpoint를 받아버리지 않게 거부 규칙을 별도로 검증한다.

FFmpeg 작업은 HTTP로 변환하지 않고 SQS를 유지한다. Istio 재시도는 멱등한 GET에만
제한적으로 검토하고 업로드 생성·완료 POST에는 자동 재시도를 적용하지 않는다.

### Mesh 보안 범위

`audio-web`과 `audio-api`에만 Sidecar를 주입한다. Istio Ingress Gateway에서 이 두
Workload로 들어가는 통신은 `PeerAuthentication`의 `STRICT` mTLS를 요구하고,
`AuthorizationPolicy`는 Ingress Gateway ServiceAccount에서 온 요청만 애플리케이션
포트로 허용한다.

Cognito JWT의 소유권과 도메인 권한 검증은 `audio-api`가 계속 수행한다. Istio
정책은 애플리케이션 인증을 대체하지 않는다. `audio-events`와
`audio-transcode`는 SQS·S3·RDS가 주요 통신 대상이므로 Sidecar를 주입하지 않고
NetworkPolicy와 AWS IAM으로 보호한다.

### Stable 라우팅 원칙

MVP는 `audio-api`의 단일 stable Deployment에 100%를 전달한다. DestinationRule과
VirtualService는 TLS와 단일 Backend 라우팅에만 사용하며 Canary subset과 weight를
만들지 않는다. Workload 이름에는 `v2`, 날짜와 `tmp`를 넣지 않는다. 트래픽 분배는
실제 버전 전환 시험이 필요해진 뒤 별도 설계와 승인으로 도입한다.

## 6. 업로드와 검사 흐름

```mermaid
sequenceDiagram
    autonumber
    actor Client as "Web Client"
    participant API as "audio-api"
    participant DB as "PostgreSQL"
    participant S3 as "S3 quarantine"
    participant Q as "scan-result Queue"
    participant Events as "audio-events"

    Client->>API: "Cognito JWT + 파일명·크기·MIME·checksum"
    API->>DB: "audio와 upload 레코드 생성"
    API-->>Client: "100 MB 이하 Presigned PUT + 필수 header"
    Client->>S3: "원본 직접 업로드"
    Client->>API: "업로드 완료 요청"
    API->>S3: "HEAD object"
    API->>DB: "크기·MIME·checksum·object version 검증 결과 저장"

    API->>S3: "CntlpScanStatus 태그 기록"
    API->>Q: "scan-result 발행 (개발용 대체 경로)"
    Events->>Q: "메시지 수신"
    Events->>DB: "중복 확인 후 scan 결과 저장"

    alt "NO_THREATS_FOUND + upload 검증 완료"
        Events->>DB: "CLEAN 전환 + Outbox 생성"
    else "THREATS_FOUND"
        Events->>DB: "QUARANTINED 전환"
    else "UNSUPPORTED·ACCESS_DENIED·FAILED"
        Events->>DB: "SCAN_FAILED 전환"
    end
```

검사 결과와 업로드 완료 요청은 실제 검사기를 도입하면 순서가 뒤바뀔 수 있다.
현재 개발용 대체 경로에서도 DB에는 `upload_verified`와 `scan_status`를 따로
저장하고 두 조건이 모두 충족됐을 때만 트랜스코딩을 요청하는 구조를 유지한다.

### 업로드 검증

- Presigned URL 만료시간은 짧게 유지한다.
- 객체 key는 서버가 생성하며 사용자가 지정하지 않는다.
- `Content-Type`과 Base64로 인코딩한 `x-amz-checksum-sha256` header를 서명
  조건에 포함한다.
- 완료 요청에서 S3 `HEAD` 결과를 다시 검사한다.
- 최대 크기 초과, checksum 불일치, 허용하지 않은 MIME은 삭제 대상으로 표시한다.
- Presigned PUT은 동일 key를 덮어쓸 수 있으므로 upload ID를 포함한 불변 key와
  S3 Versioning을 사용한다.

클라이언트가 전달한 확장자와 MIME만으로 파일 형식을 신뢰하지 않는다. Worker는
FFprobe로 실제 미디어 형식과 duration을 확인한다.

## 7. 트랜스코딩과 파형 생성

```mermaid
sequenceDiagram
    autonumber
    participant Outbox as "Outbox Publisher"
    participant Q as "transcode Queue"
    participant Worker as "audio-transcode"
    participant Source as "S3 quarantine"
    participant Artifact as "S3 artifacts"
    participant Result as "transcode-result Queue"
    participant Events as "audio-events"
    participant DB as "PostgreSQL"

    Outbox->>Q: "TranscodeRequested"
    Worker->>Q: "long polling으로 메시지 수신"
    Worker->>Source: "CLEAN tag가 있는 원본 다운로드"
    Worker->>Worker: "FFprobe 제한 검증"
    Worker->>Worker: "FFmpeg MP3 변환"
    Worker->>Worker: "waveform peak JSON 생성"
    Worker->>Artifact: "playback.mp3 + waveform.json 업로드"
    Worker->>Result: "TranscodeSucceeded"
    Worker->>Q: "원본 메시지 삭제"
    Events->>Result: "결과 수신"
    Events->>DB: "artifact와 READY 상태 저장"
```

MVP 기본 출력은 다음과 같다.

| Artifact | 기본값 | 용도 |
| --- | --- | --- |
| `playback.mp3` | stereo, 192 kbps | 브라우저 재생 |
| `waveform.json` | mono, 8-bit min/max peaks | 전체 파형 렌더링 |

구체적인 bitrate와 peak 해상도는 부하·품질 실험 결과로 변경할 수 있다.

### 파형 처리

파형은 재생 중 생성하지 않는다. Worker가 전체 오디오의 일정 구간별 min/max
amplitude를 미리 계산한다.

```json
{
  "schema_version": 1,
  "duration_ms": 185430,
  "points_per_second": 20,
  "bits": 8,
  "channels": 1,
  "peaks": [[-2, 3], [-5, 8], [-12, 15]]
}
```

브라우저는 JSON을 한 번 받아 Canvas나 SVG로 렌더링하고
`HTMLMediaElement.currentTime / duration` 비율로 재생 커서를 이동한다.
파형을 클릭하면 같은 비율로 `currentTime`을 변경한다.

JSON은 원본 PCM sample을 저장하지 않는다. 초당 peak 수와 전체 peak 개수를
제한하고 CloudFront 압축을 사용한다. 확대 편집 기능이 필요해질 때만
다중 해상도 또는 binary waveform을 추가한다.

## 8. 재생 흐름

```mermaid
sequenceDiagram
    autonumber
    actor Client as "Web Client"
    participant API as "audio-api"
    participant DB as "PostgreSQL"
    participant CF as "CloudFront"
    participant S3 as "Private S3 artifacts"

    Client->>API: "GET /v1/audios/{audio_id}/playback"
    API->>DB: "READY와 visibility·소유권 확인"
    API-->>Client: "MP3와 waveform Signed URL"
    Client->>CF: "waveform.json 요청"
    CF->>S3: "OAC로 조회"
    Client->>CF: "playback.mp3 Range GET"
    CF->>S3: "필요한 byte range 조회"
    Client->>Client: "파형 렌더링 + currentTime 커서 동기화"
```

- 업로드와 내부 S3 작업에는 Presigned S3 URL을 사용한다.
- 사용자 재생에는 CloudFront URL을 사용한다.
- 공개 오디오도 API가 visibility를 확인한 후 짧은 URL을 발급할 수 있다.
- 비공개 오디오는 Cognito 사용자와 owner를 확인한다.
- 원본 파일은 어떤 visibility에서도 재생 URL을 발급하지 않는다.

MVP에서는 MP3 한 객체에 대한 Range GET으로 seek한다. HLS는 실제 측정에서
Adaptive Bitrate, 라이브 스트리밍 또는 복구성이 필요하다고 확인될 때 도입한다.

## 9. 상태 모델

```mermaid
stateDiagram-v2
    [*] --> UPLOAD_PENDING
    UPLOAD_PENDING --> UPLOADED: "HEAD 검증 완료"
    UPLOAD_PENDING --> UPLOAD_EXPIRED: "만료"
    UPLOADED --> SCANNING
    SCANNING --> CLEAN: "NO_THREATS_FOUND"
    SCANNING --> QUARANTINED: "THREATS_FOUND"
    SCANNING --> SCAN_FAILED: "UNSUPPORTED / ACCESS_DENIED / FAILED"
    CLEAN --> QUEUED: "Outbox 발행"
    QUEUED --> TRANSCODING: "Worker 시작"
    TRANSCODING --> READY: "MP3와 waveform 완료"
    TRANSCODING --> QUEUED: "재시도 가능 실패"
    TRANSCODING --> TRANSCODE_FAILED: "최대 재시도 초과"
    READY --> DELETED: "사용자 삭제"
    QUARANTINED --> DELETED: "보존기간 만료"
```

보안 검사 실패는 fail-open하지 않는다. `UNSUPPORTED`, `ACCESS_DENIED`,
`FAILED`도 CLEAN으로 간주하지 않는다.

MP3 생성은 성공했지만 waveform만 실패한 경우 같은 Job을 재시도한다. 최대
재시도 후에는 MP3를 보존하되 `TRANSCODE_FAILED`로 두고 운영자가 재처리할 수
있게 한다. MVP에서 불완전한 artifact를 `READY`로 공개하지 않는다.

## 10. API 경계

| Method | Path | 인증 | 책임 |
| --- | --- | --- | --- |
| `POST` | `/v1/audios/uploads` | Cognito JWT | audio ID와 Presigned PUT 생성 |
| `POST` | `/v1/audios/{audio_id}/complete` | Cognito JWT | S3 객체 검증 완료 기록 |
| `GET` | `/v1/audios/{audio_id}` | visibility에 따라 선택 | 메타데이터와 처리 상태 조회 |
| `GET` | `/v1/audios/{audio_id}/playback` | visibility에 따라 선택 | MP3·waveform CloudFront URL 발급 |
| `PATCH` | `/v1/audios/{audio_id}/visibility` | owner JWT | 공개 범위 변경 |
| `DELETE` | `/v1/audios/{audio_id}` | owner JWT | 논리 삭제와 객체 정리 요청 |

`audio-api`만 PostgreSQL의 애플리케이션 테이블을 직접 소유한다.
`audio-events`는 같은 Go 애플리케이션의 별도 실행 모드로 DB 트랜잭션을 사용한다.
Python Worker는 RDS에 연결하지 않고 S3와 SQS만 사용한다.

## 11. 데이터 모델

### `audios`

| 필드 | 의미 |
| --- | --- |
| `id` | 서버가 발급한 audio ID |
| `owner_subject` | Cognito `sub` |
| `title` | 사용자 표시 제목 |
| `visibility` | `public` 또는 `private` |
| `status` | 오디오 전체 상태 |
| `upload_id` | 업로드 재사용 방지 ID |
| `source_bucket`, `source_key`, `source_version` | 원본 위치 |
| `source_checksum`, `source_size`, `source_mime` | 검증 정보 |
| `scan_status` | 원본 검사 결과 |
| `duration_ms` | FFprobe 결과 |
| `playback_key`, `waveform_key` | artifact 위치 |
| `created_at`, `updated_at`, `deleted_at` | 수명주기 |

### `transcode_jobs`

| 필드 | 의미 |
| --- | --- |
| `id` | 전역 고유 job ID |
| `audio_id` | 대상 오디오 |
| `status` | `QUEUED`, `RUNNING`, `SUCCEEDED`, `FAILED` |
| `target_spec` | MP3와 waveform 생성 규격 |
| `attempt_count` | 애플리케이션 재시도 횟수 |
| `error_code` | 제한된 오류 분류 |
| `started_at`, `finished_at` | 처리 시간 |

### `outbox_events`

| 필드 | 의미 |
| --- | --- |
| `id` | event ID |
| `aggregate_id` | audio 또는 job ID |
| `event_type` | `TranscodeRequested` 등 |
| `schema_version` | 메시지 계약 버전 |
| `payload` | Queue에 보낼 작은 JSON |
| `created_at`, `published_at` | 발행 상태 |

### `processed_events`

`audio-events`가 이미 처리한 SQS event ID를 기록한다. 같은 이벤트가
재전달되면 DB 상태를 다시 변경하지 않는다.

## 12. Queue와 메시지 계약

| Queue | Producer | Consumer | DLQ |
| --- | --- | --- | --- |
| `scan-result` | `audio-api`의 Scan Adapter | `audio-events` | `scan-result-dlq` |
| `transcode` | Outbox Publisher | `audio-transcode` | `transcode-dlq` |
| `transcode-result` | `audio-transcode` | `audio-events` | `transcode-result-dlq` |

클라우드 자원 이름 예시는 다음 패턴을 따른다.

```text
cntlp-aws-queue-scan-result
cntlp-aws-queue-scan-result-dlq
cntlp-aws-queue-transcode
cntlp-aws-queue-transcode-dlq
cntlp-aws-queue-transcode-result
cntlp-aws-queue-transcode-result-dlq
```

`shared/schema/transcode-job.json`의 목표 필드는 다음과 같다.

```json
{
  "schema_version": 1,
  "event_id": "uuid",
  "job_id": "uuid",
  "audio_id": "uuid",
  "source": {
    "bucket": "cntlp-aws-quarantine",
    "key": "incoming/...",
    "version_id": "...",
    "checksum_sha256": "..."
  },
  "targets": {
    "mp3_bitrate_kbps": 192,
    "waveform_points_per_second": 20
  }
}
```

Queue에는 오디오 binary, 사용자 개인정보, Presigned URL과 AWS 자격 증명을 넣지
않는다.

### 전달 보장과 멱등성

- SQS Standard의 중복과 순서 변경을 정상 상황으로 취급한다.
- `(job_id, schema_version)`은 DB에서 유일해야 한다.
- artifact key는 job ID로 결정해 재실행 결과가 같은 위치에 기록되게 한다.
- Worker는 성공 결과 메시지를 보낸 뒤 원본 작업 메시지를 삭제한다.
- 결과 전송 후 삭제 전에 Worker가 죽어도 재실행이 안전해야 한다.
- 처리 시간이 길면 Worker가 SQS visibility timeout을 연장한다.
- DLQ redrive는 자동 무한 반복하지 않고 오류 원인을 확인한 뒤 수행한다.

## 13. DB와 Queue 일관성

API 요청에서 DB를 갱신한 직후 Queue 전송이 실패하면 상태와 메시지가 어긋난다.
이를 막기 위해 같은 PostgreSQL 트랜잭션에서 상태 변경과 Outbox row 생성을 함께
커밋한다.

```text
BEGIN
  audio 상태 CLEAN → QUEUED
  transcode_job 생성
  outbox_event 생성
COMMIT

Outbox Publisher
  → SQS 전송
  → published_at 기록
```

SQS 전송 성공 후 `published_at` 기록 전에 프로세스가 종료되면 메시지가 중복
발행될 수 있다. Worker의 job ID 멱등성이 이를 흡수한다.

## 14. S3 구조와 접근 제어

### Bucket

| 이름 예시 | 내용 | 사용자 직접 읽기 |
| --- | --- | --- |
| `cntlp-aws-quarantine` | 업로드 원본과 격리 대상 | 금지 |
| `cntlp-aws-transcode` | MP3와 waveform artifact | CloudFront를 통해서만 허용 |

```text
cntlp-aws-quarantine/
└─ incoming/{audio_id}/{upload_id}/source

cntlp-aws-transcode/
└─ audios/{audio_id}/artifacts/{job_id}/
   ├─ playback.mp3
   └─ waveform.json
```

### 정책

- S3 Block Public Access를 켠다.
- Bucket owner enforced를 사용하고 ACL을 사용하지 않는다.
- 기본 암호화를 켠다.
- 원본 객체에 검사 상태 태그를 기록한다. 현재 기록 주체는 개발용 `audio-api`다.
- CloudFront는 OAC를 통해 artifact bucket의 `GetObject`만 허용한다.
- Transcode Worker는 `CntlpScanStatus` tag가 `NO_THREATS_FOUND`인 원본만 읽는다.
- 감염 원본과 완료되지 않은 업로드에는 Lifecycle 보존기간을 적용한다.
- 원본과 artifact 삭제는 DB 논리 삭제 후 비동기 정리로 처리한다.

## 15. 인증과 AWS 권한

### 사용자 인증

- Web은 Cognito Authorization Code + PKCE 흐름을 사용한다.
- API는 Cognito JWKS로 JWT 서명을 검증한다.
- `sub`, `iss`, `aud` 또는 `client_id`, 만료시간을 확인한다.
- 이메일을 DB의 불변 소유자 키로 사용하지 않는다.

### Workload 권한

장기 IAM access key를 Kubernetes Secret, Git, Terraform 변수에 저장하지 않는다.
목표 구조는 self-managed Kubernetes ServiceAccount와 AWS IAM OIDC federation을
연결해 임시 자격 증명을 발급하는 것이다.

| ServiceAccount | 최소 권한 |
| --- | --- |
| `audio-api` | Presigned PUT, CloudFront 서명키 조회, 필요한 Secret 조회 |
| `audio-events` | 세 Queue consume/publish, 필요한 Secret 조회 |
| `audio-transcode` | CLEAN 원본 읽기, artifact 쓰기, 작업·결과 Queue 접근 |
| `cert-manager` | 지정 Hosted Zone의 DNS-01 검증 레코드 생성·조회·삭제 |

자동 확장을 선택할 때만 `keda-operator`의 Queue 조회 권한과
`cluster-autoscaler`의 ASG 조회·용량 변경·인스턴스 종료 권한을 추가한다.

OIDC federation 도입 전 임시로 EC2 instance profile을 사용한다면 Workload를
전용 AWS Node에 고정하고 역할을 최소화한다. 정적 IAM user key는 대안으로
허용하지 않는다.

RDS 비밀번호와 CloudFront private signing key의 원본은 Secrets Manager에 두고,
Git과 매니페스트에는 넣지 않는다.

### Worker 실행 격리

검사에서 악성코드가 나오지 않았다는 결과가 미디어 decoder 취약점까지 없다는
뜻은 아니다. 현재는 실제 검사기 자체가 없으므로 이 제한의 비중이 더 크다.
FFmpeg Worker는 다음 제한을 적용한다.

- non-root 사용자로 실행
- `allowPrivilegeEscalation: false`
- `readOnlyRootFilesystem: true`
- Linux capability 전체 제거
- `seccompProfile: RuntimeDefault`
- ServiceAccount token 자동 mount 비활성화
- CPU·메모리·ephemeral storage 제한
- 입력·출력 경로 외 hostPath와 host network 사용 금지

## 16. Worker 실행과 확장 판단

MVP 기준은 정적 Worker다. 자동 확장은 필수 구성요소가 아니라 부하와 비용 측정
후 비교할 선택 후보이다.

### MVP 기준: 정적 Worker

- 기존 AWS Worker를 사용한다.
- `audio-transcode`는 replica 1인 Deployment로 실행한다.
- SQS long polling으로 메시지를 가져오고 성공한 메시지만 삭제한다.
- `audio-transcode` 동시 처리 수는 1이다.
- 파일 최대 크기는 100 MB다.
- CPU·메모리 request와 limit, 임시 디스크 limit을 측정한다.

### 선택 후보 1: KEDA

다음 조건이 확인될 때 KEDA ScaledObject 또는 ScaledJob을 비교한다.

- 정적 Worker에서 SQS oldest message age가 목표 시간을 반복해 초과한다.
- 수동 replica 조정으로는 부하 실험을 재현하기 어렵다.
- Queue visible·in-flight 메시지를 구분한 확장 기준을 정의할 수 있다.
- `maxReplicaCount`와 최대 동시 처리 비용을 승인받았다.

유휴 Pod를 0으로 줄이는 장점만으로 KEDA를 도입하지 않는다. 장시간 실행하는
Deployment와 메시지당 Job 중 처리량·시작 지연·실패 복구가 나은 방식을 측정한다.

### 선택 후보 2: AWS Worker ASG

KEDA 검증 후에도 Node 용량이 병목이고 유휴 Node 비용을 줄여야 할 때만 ASG와
Cluster Autoscaler를 검토한다. 선택한다면 초기 상한은 `min=0`, `max=1`이며,
예산 승인 전에는 확대하지 않는다. scale-from-zero에는 다음 Node template
label이 필요하다.

```text
k8s.io/cluster-autoscaler/enabled
k8s.io/cluster-autoscaler/cntlp-k8s
k8s.io/cluster-autoscaler/node-template/label/platform=aws
k8s.io/cluster-autoscaler/node-template/label/role=service
```

실제 Node bootstrap도 같은 `platform=aws`, `role=service` 라벨을 적용해야 한다.

### ASG 선택 전 Node bootstrap 전제

ASG를 활성화하기 전에 다음 자동화가 검증되어야 한다.

1. Ansible 역할을 사용해 Packer로 Worker AMI 생성
2. 부팅 시 IMDSv2 instance profile로 필요한 bootstrap secret만 조회
3. Tailscale에 `tag:cntlp-wk`로 가입
4. Tailscale IPv4를 kubelet `--node-ip`로 지정
5. 회전 가능한 kubeadm join credential로 `cntlp-k8s` 가입
6. Node Ready와 Calico 준비 상태 확인
7. 종료 시 Pod drain, Kubernetes Node와 Tailscale device 정리

이 절차가 없으면 ASG는 EC2만 늘리고 Kubernetes Worker를 만들지 못한다.
따라서 ASG는 별도 인프라 승인과 장애 복구 시험 후에만 활성화한다. 조건을
충족하지 않으면 정적 Worker 구성을 유지한다.

## 17. 실패 처리

| 실패 | 처리 | 사용자 상태 |
| --- | --- | --- |
| Presigned URL 만료 | 새 upload ID 발급 | `UPLOAD_PENDING` |
| 업로드 크기·checksum 불일치 | 객체 삭제 대상으로 표시 | `UPLOAD_FAILED` |
| 검사에서 위협 발견 | 트랜스코딩 금지, 격리 보존 | `QUARANTINED` |
| 검사 불가 | 자동 통과 금지, 운영 확인 | `SCAN_FAILED` |
| SQS 중복 메시지 | event ID·job ID로 무시 또는 안전 재실행 | 기존 상태 유지 |
| Worker Pod 종료 | visibility timeout 후 재전달 | `QUEUED` 또는 `TRANSCODING` |
| FFmpeg 제한 위반 | 비재시도 오류로 DLQ | `TRANSCODE_FAILED` |
| 일시적 S3·SQS 오류 | 지수 backoff 후 제한 재시도 | 처리 중 |
| 결과 메시지 유실 | Worker가 작업 메시지를 삭제하지 않아 재실행 | 처리 중 |
| DLQ 적체 | 경보 후 원인 확인·수동 redrive | 실패 상태 |
| 정적 Worker 용량 부족 | Queue 경보 후 replica·Node를 수동 조정 | `QUEUED` |
| 자동 확장 실험 실패 | 확장을 중지하고 정적 Worker로 복귀 | `QUEUED` |
| NLB Target 비정상 | Target health 경보, fail-open 가능성 포함 Gateway 상태 확인 | 처리 중인 비동기 작업 유지 |
| Istio Gateway 준비 실패 | NLB health check 실패, Gateway 롤백 | 외부 API 일시 중단 |
| 인증서 발급·갱신 실패 | 만료일 경보, 기존 Secret 유지 후 원인 복구 | 기존 인증서 유효기간 내 유지 |
| Istio route 오류 | stable 100% 기준 설정으로 롤백 | 외부 API 일시 중단 가능 |

FFmpeg와 FFprobe에는 실행 시간, CPU, 메모리, 임시 디스크, 출력 크기 제한을 둔다.
사용자가 올린 파일 이름을 shell command에 직접 보간하지 않고 argv로 전달한다.

## 18. 관측성과 FinOps

### Metrics

- API 요청 수·오류율·latency
- NLB active flow·new flow·healthy target 수
- Istio Gateway 요청 수·응답 코드·p50/p95 latency
- `audio-api` 요청 수·오류율·p50/p95 latency
- Istio Proxy·Gateway CPU·메모리와 재시작 횟수
- TLS 인증서 만료까지 남은 시간
- upload 완료율과 크기 histogram
- scan 결과별 개수
- SQS visible·in-flight·oldest message age
- transcode queue time·processing time·success·failure
- Worker CPU·메모리·임시 디스크
- 자동 확장 실험 시 KEDA desired 수와 실제 Pod·Job 수
- ASG 실험 시 Node 수와 scale event
- MP3 byte 전송량과 CloudFront cache hit ratio
- S3 저장량·GET·PUT 요청 수
- 오디오 1건당 처리 시간과 추정 비용

`audio_id`, `job_id`, `user_id`는 로그와 trace field에는 사용할 수 있지만
Prometheus label이나 Kubernetes label에는 사용하지 않는다.

### 로그와 추적

- 로그는 JSON 구조로 남긴다.
- event ID, audio ID, job ID를 correlation field로 사용한다.
- Presigned URL, JWT, CloudFront signature, AWS credential은 로그에 남기지 않는다.
- VPC Flow Logs·NLB CloudWatch 지표와 Istio access log를 같은 시간축에서 조회할 수
  있게 UTC timestamp를 유지한다.
- Istio trace와 애플리케이션 trace는 같은 trace context를 사용한다.
- API → Outbox → SQS → Worker → Result 흐름의 시간 구간을 분리해 측정한다.

## 19. 비용 경계

- 원본과 artifact를 서로 다른 버킷에 저장해 IAM, Lifecycle과 버킷별 저장량 측정을
  분리한다. 버킷 분리는 측정 경계를 만들며 그 자체로 저장 비용을 줄이지 않는다.
- `cntlp-aws-quarantine`의 128 KB 초과 원본은 0~29일 Standard, 30~59일
  Standard-IA, 60일 이후 Glacier Instant Retrieval을 예상 기준선으로 둔다.
- `cntlp-aws-transcode`는 사용자 재생 경로이므로 MVP에서 Lifecycle을 적용하지 않는다.
- 실제 접근 로그와 재처리 빈도를 확인하기 전에는 Lifecycle 절감액을 예상값으로
  표시하고 실제 AWS 청구 절감액으로 표현하지 않는다.
- artifact는 MP3 한 rendition과 waveform 하나로 시작한다.
- HLS segment와 320 kbps 추가 rendition은 측정 근거 없이 생성하지 않는다.
- 자동 확장을 선택하면 KEDA와 ASG의 최대 동시 처리 수를 함께 제한한다.
- scale-from-zero 실험에서 Node bootstrap 실패가 반복되면 자동 확장을 정지하고
  정적 Worker로 복귀한다.
- DLQ 메시지는 비용과 장애 신호이므로 오래 방치하지 않는다.
- CloudFront와 S3 request·transfer 비용을 MP3 기준선으로 기록한다.
- Internet-facing NLB 시간·용량 단위 비용을 고정 인프라 비용에 포함한다.
- Istio Gateway와 Proxy의 request·사용량은 앱과 분리해 OpenCost에서 표시한다.
- `audio-transcode`에는 Sidecar를 주입하지 않아 트랙당 Worker 비용에 Mesh 비용을 섞지 않는다.
- cert-manager와 DNS 운영 비용은 공개 HTTPS 비용으로 분리한다.
- NLB에는 AWS WAF를 직접 연결할 수 없다는 제약을 수용한다. L7 WAF가 필수가 되면
  CloudFront + WAF 추가 또는 ALB 재검토를 별도 결정으로 남긴다.

## 20. 저장소별 구현 경계

### `00-cantaloupe-resources`

- 프로젝트 공통 명명·라벨 규칙
- 여러 저장소에 공통으로 적용되는 보안·비용·운영 정책

### `01-infra-provisioning`

- `terraform/aws/network`: VPC와 Worker Security Group 자체
- `terraform/aws/edge`: Internet-facing NLB, NLB Security Group, Worker NodePort
  ingress rule, Listener, Target Group, Target 등록과 Public DNS alias
- `terraform/aws/edge`: cert-manager DNS-01용 최소 IAM과 관련 출력
- Edge는 Network·Compute remote state를 읽고 Worker instance ID 변경을 Target
  attachment에 반영
- S3, SQS, DLQ, Cognito, CloudFront, IAM
- 자동 확장 선택 시 ASG와 Packer·Ansible Worker image·bootstrap
- Terraform 비용 상한과 destroy 경계

### `02-k8s-manifests`

- Istio Helm chart 버전 고정, control plane과 `istio-ingress` NodePort Service
- Istio 업그레이드 전 diff·분석과 이전 버전 rollback 절차
- cert-manager Certificate·Issuer 참조와 TLS Secret 연결
- Gateway, VirtualService, DestinationRule
- PeerAuthentication, AuthorizationPolicy와 선택적 Sidecar 주입
- `apps` Namespace의 Deployment·Service
- AWS Node selector
- resource request·limit
- NetworkPolicy, PodDisruptionBudget, Secret 참조
- 자동 확장 선택 시 KEDA와 Cluster Autoscaler 설정

매니페스트는 GitOps 저장소를 통해 배포하고 수동 `kubectl apply`를 기준 절차로
사용하지 않는다.

### `03-app-audio`

- 이 서비스 아키텍처와 변경 이유
- API·Queue 메시지 계약
- Go `audio-api`와 `audio-events`
- Python `audio-transcode`
- Web UI
- DB migration
- `shared/schema`의 Queue JSON Schema
- 단위·통합·FFmpeg fixture 테스트

API와 Worker 메시지 Schema는 같은 커밋에서 호환되게 변경한다.

## 21. 단계별 구현

### Phase 1: 계약과 로컬 수직 흐름

1. `transcode-job.json`에 schema version과 job ID 추가
2. PostgreSQL migration 작성
3. Go upload·complete·status API 작성
4. Python MP3·waveform Worker 작성
5. Local PostgreSQL·S3·SQS 대체 환경에서 한 건 처리
6. 중복 메시지와 Worker 강제 종료 시험
7. Mock 데이터 없는 Web 앱 셸·Presigned 업로드·상태 polling 구현

### Phase 2: AWS 관리형 자원 연결

1. Private S3와 Lifecycle
2. SQS 세트와 DLQ
3. Cognito
4. CloudFront OAC와 Signed URL
5. Private RDS와 migration
6. Internet-facing NLB, Target Group, Security Group과 Public DNS
7. cert-manager DNS-01용 IAM

### Phase 3: Kubernetes 배포

1. Istio control plane과 Ingress Gateway 배포
2. Gateway NodePort와 NLB Target health 검증
3. cert-manager 인증서 발급과 HTTPS·HTTP redirect 검증
4. `audio-web`, `audio-api`, `audio-events` 배포
5. `audio-web`, `audio-api` Sidecar와 STRICT mTLS 적용
6. AuthorizationPolicy와 기본 stable 100% route 적용
7. AWS Node selector, resource limit, NetworkPolicy와 Secret 연동
8. replica 1인 `audio-transcode` Deployment 적용
9. 실제 SQS·S3·RDS 통합 검증

### Phase 4: Service Mesh 검증

1. NLB → Gateway → Web·API 경로와 Source IP 확인
2. 외부 `/metrics`·`/healthz` 차단과 내부 probe 확인
3. Gateway → Web·API mTLS와 비인가 Workload 차단 확인
4. 단일 Backend HTTP routing과 Source IP 전달 확인
5. Istio 적용 전후 latency·CPU·메모리·OpenCost 비교

Canary 트래픽 분배와 오류 주입은 애플리케이션 버전 전환 시험이 필요할 때 후속
검증으로 수행한다.

### Phase 5: 자동 확장 채택 판단

1. 정적 Worker의 Queue 대기시간·처리량·유휴 비용 측정
2. 필요하면 KEDA ScaledObject와 ScaledJob 비교 실험
3. Node 용량이 병목일 때만 Packer Worker AMI와 bootstrap 검증
4. ASG를 선택하면 `min=0`, `max=1`로 scale-from-zero 시험
5. 채택 근거와 비용 상한을 별도 결정으로 기록

### Phase 6: 부하·FinOps 실험

1. Raw·Processed 버킷의 실제 저장량과 객체 수 측정
2. Lifecycle 미적용·적용 장기 예상 비용과 절감률 비교
3. MP3 기준 Worker CPU·메모리·처리시간 측정
4. Worker requests·limits Right-sizing 전후 처리시간·실패율 비교
5. 같은 실험 구간의 OpenCost Worker 할당 비용을 정상 완료 건수로 나눈 트랙당 비용 계산
6. Queue 적체와 정적 Worker 처리량·장애·DLQ·복구 시험
7. 자동 확장을 선택한 경우에만 KEDA·ASG 비용과 복구 시험
8. 필요할 때만 HLS 비교 구현

## 22. 완료 기준

- 브라우저가 AWS credential 없이 100 MB 이하 오디오를 직접 업로드한다.
- 악성코드 검사 완료 전에는 Worker와 사용자가 원본을 읽지 못한다.
- 위협·검사 실패 파일이 재생 가능한 상태로 전환되지 않는다.
- 같은 SQS 메시지를 두 번 처리해도 DB와 artifact가 중복 생성되지 않는다.
- Worker가 중간에 종료되어도 메시지가 다시 처리된다.
- MP3 Range GET seek와 waveform click seek가 동작한다.
- Public DNS의 HTTPS가 NLB와 Istio Gateway를 통해 Web·API에 도달한다.
- HTTP 요청은 Istio Gateway에서 HTTPS로 리다이렉트된다.
- NLB Target health가 Gateway readiness와 일치하고 NodePort 직접 접근은 차단된다.
- Gateway에서 `audio-web`과 `audio-api`까지 STRICT mTLS가 적용된다.
- 비인가 ServiceAccount는 Mesh 대상 Workload에 접근하지 못한다.
- Gateway가 단일 Backend로 요청을 전달하고 mTLS·인가 정책을 적용한다.
- API와 Worker는 GCP·On-Prem Node에 배치되지 않는다.
- Queue가 비면 정적 Worker가 추가 작업 없이 long polling 상태를 유지한다.
- 파일 1건 기준 처리 시간·오류·AWS 사용량을 관측할 수 있다.
- 모든 Secret과 장기 credential이 Git·이미지·Queue·로그에 없다.

KEDA나 ASG를 채택하면 scale-out, scale-in, bootstrap 실패와 비용 상한을 별도 완료
기준으로 추가한다. 자동 확장을 선택하지 않아도 위 MVP 완료 기준은 충족할 수 있다.

## 23. 선택하지 않은 대안

### 오디오 binary를 PostgreSQL에 저장

DB 백업, I/O, 용량과 네트워크 비용을 키우므로 선택하지 않는다. DB에는
메타데이터와 상태만 저장한다.

### API를 통한 multipart binary 업로드

API Pod의 메모리·네트워크를 파일 전달에 사용하고 확장 병목을 만들므로 선택하지
않는다. Client가 S3에 직접 업로드한다.

### Redis·NATS를 클러스터에서 운영

현재 목표에는 SQS가 제공하는 Queue와 DLQ로 충분하다. 메시징 서버의 메모리,
볼륨, 백업과 장애 복구를 추가하지 않는다.

### ClamAV Pod

악성코드 엔진과 signature 갱신을 직접 운영하고 현재 AWS Worker 자원을
소비하므로 MVP에서 제외한다. 단일 AWS Service Worker에서 web·api·worker를 함께
실행하는 구성에서는 signature DB 상주 메모리만으로 기준선이 깨진다. 관리형
대안인 GuardDuty도 계정 플랜 제약으로 쓸 수 없어, MVP는 검사기 없이 계약만
유지한다. 두 방식의 비교는 별도 실험으로 남긴다.

### HLS

VOD 오디오 한 rendition에는 playlist·segment·인증·CORS·요청 수 증가가 이점보다
크다. Progressive MP3와 Range GET을 기준선으로 삼고 Adaptive Bitrate나 라이브
요구가 확인될 때 도입한다.

### Worker의 RDS 직접 연결

Worker가 DB credential과 schema에 결합되고 Node가 늘어날수록 DB 접근 경계가
넓어진다. Worker는 결과 Queue만 발행하고 `audio-events`가 DB를 갱신한다.

### ALB에서 직접 애플리케이션으로 라우팅

ALB는 ACM TLS 종료, 경로 라우팅과 WAF 연동을 단순하게 제공하지만 Service Mesh의
Ingress와 mTLS 정책 경계를 ALB 뒤로 밀어낸다. 이 설계는 NLB를 L4 전달에만 사용하고
Istio가 TLS와 L7 정책을 소유하도록 선택한다. 대신 Gateway·인증서 운영 비용과
NLB의 AWS WAF 직접 연동 불가를 명시적인 트레이드오프로 수용한다.

## 24. 참고 문서

- [Amazon S3 Presigned URL](https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-presigned-url.html)
- [Amazon SQS 선택 가이드](https://docs.aws.amazon.com/pdfs/decision-guides/latest/sns-or-sqs-or-eventbridge/sns-or-sqs-or-eventbridge.pdf)
- [CloudFront Range GET](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/RangeGETs.html)
- [CloudFront S3 OAC](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/private-content-restricting-access-to-s3.html)
- [AWS Network Load Balancer](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/introduction.html)
- [NLB Target Group과 Health Check](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/target-group-health-checks.html)
- [NLB TCP TLS Pass-through](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/load-balancer-listeners.html)
- [NLB Security Group](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/load-balancer-security-groups.html)
- [AWS WAF 연결 가능 자원](https://docs.aws.amazon.com/waf/latest/developerguide/web-acl-associating-aws-resource.html)
- [Istio Ingress Gateway](https://istio.io/latest/docs/tasks/traffic-management/ingress/)
- [Istio Secure Gateway](https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/)
- [Istio PeerAuthentication](https://istio.io/latest/docs/reference/config/security/peer_authentication/)
- [Istio Sidecar Injection](https://istio.io/latest/docs/setup/additional-setup/sidecar-injection/)
- [Istio Traffic Shifting](https://istio.io/latest/docs/tasks/traffic-management/traffic-shifting/)
- [cert-manager DNS-01](https://cert-manager.io/docs/configuration/acme/dns01/)
- [KEDA SQS Scaler](https://keda.sh/docs/2.20/scalers/aws-sqs/)
- [KEDA ScaledJob](https://keda.sh/docs/2.20/concepts/scaling-jobs/)
- [Kubernetes Node Autoscaling](https://kubernetes.io/docs/concepts/cluster-administration/node-autoscaling/)
- [Cluster Autoscaler on AWS](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/cloudprovider/aws/README.md)
- [BBC audiowaveform](https://github.com/bbc/audiowaveform)
- [Web MP3 codec compatibility](https://developer.mozilla.org/en-US/docs/Web/Media/Guides/Formats/Audio_codecs)
