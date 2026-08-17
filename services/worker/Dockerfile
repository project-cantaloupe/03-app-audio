# 이 이미지는 ffmpeg 때문에 배포판을 고른다. 파이썬 런타임은 부차적이다.
#
# 왜 Debian 이 아닌가 — 버전 문제가 아니었다. bookworm(ffmpeg 5.1) 에서 560건이
# 나와 trixie(ffmpeg 7.1.5) 로 옮겼는데 564건이 나왔다. 유지보수되는 최신 라인을
# 깔고도 그대로였다는 뜻이다. 564건 중 수정 가능은 66건뿐이고, 나머지 ~500건은
# Debian 이 no-dsa/unimportant 로 분류해 **패치를 발행하지 않는** libav* CVE 다.
# apt 로 닿을 수 있는 곳에 답이 없으므로 Debian 계열에 apt install ffmpeg 를 하는
# 한 deployment_security=high 게이트는 어떤 스위트에서도 통과하지 못한다.
#
# Alpine 은 두 가지가 다르다.
#   1. 실제로 더 새 ffmpeg 를 준다 — community/v3.23 이 8.0.1-r1
#   2. secdb 가 **수정된 CVE만** 싣는다. Debian 이 "영영 안 고칠 것"까지 신고하는
#      반면 Alpine 은 미수정 건을 아예 데이터에 넣지 않는다
# 개선분에는 (1)의 진짜 갱신과 (2)의 데이터 소스 효과가 섞여 있다. 보고 숫자가
# 0에 가까워져도 실제 공격면이 그만큼 줄어드는 것은 아니다. 이 이미지를 볼 때
# 스캔 0건을 "ffmpeg 에 취약점이 없다"로 읽지 않는다.
#
# 전환 비용이 낮은 근거 — 런타임 의존성은 boto3 와 prometheus-client 둘뿐이고
# 의존 그래프가 전부 순수 파이썬 휠이다. musl 전환의 통상적 지뢰인 네이티브
# 확장 빌드가 없다.
#
# alpine3.23 을 태그에 박는다. `-alpine` 만 쓰면 배포판이 제 일정으로 흘러가고,
# 그 변화가 무관한 빌드에 예고 없이 도착한다.
FROM python:3.13-alpine3.23 AS runtime

# 패키지 버전은 고정하지 않는다 — 옛 apt 레이어와 같은 기준이다. 고정 대상은
# 배포판(위 태그의 alpine3.23)이고, 그 안에서 나오는 보안 업데이트는 재빌드로
# 자동으로 따라오게 둔다. `ffmpeg=8.0.1-r1` 로 못박으면 Alpine 이 -r2 를 내며
# APKINDEX 에서 구버전을 걷어내는 날 빌드가 통째로 깨지고, 패치를 받는 데 매번
# 사람 손이 필요해진다. 이 이미지가 게이트에 막혀 있던 원인이 바로 그 지연이었다.
#
# 계정 생성이 Debian 의 groupadd/useradd 와 다른 것은 문법만이 아니다. Alpine 의
# adduser 는 busybox 애플릿이라 GNU 롱옵션이 없고, `-S` 하나가 시스템 계정 ·
# nologin 셸 · 잠긴 패스워드를 한꺼번에 뜻한다. UID/GID 10001 은 K8s
# securityContext 의 runAsUser/runAsGroup 과 짝이므로 바꾸지 않는다.
RUN apk add --no-cache ffmpeg \
    && addgroup -S -g 10001 cntlp \
    && adduser -S -u 10001 -G cntlp -h /app -D cntlp

WORKDIR /app

COPY pyproject.toml ./
COPY src ./src

# 설치가 끝나면 pip 을 걷어낸다. Alpine 전환 뒤 게이트에 마지막까지 남았던 3건이
# 전부 pip 안에서 나왔다(`b994a23` 실측, Trivy v0.72.0).
#
#   setuptools 70.3.0 — CVE-2025-47273(High) · CVE-2026-59890(Medium)
#   msgpack    1.1.2  — GHSA-6v7p-g79w-8964(High)
#
# 둘 다 우리가 설치한 것이 아니다. 같은 빌드의 pip 출력이 실제로 넣은 것은
# boto3 · botocore · jmespath · prometheus-client · python-dateutil ·
# s3transfer · six · urllib3 뿐이고 여기에 setuptools 도 msgpack 도 없다.
# 정체는 pip 이 자기 안에 들고 다니는 벤더 사본(pip/_vendor/)이다 — Trivy 가
# 그 vendor 목록을 독립된 패키지로 읽어 보고한다. 그래서 "설치한 적 없는
# 패키지"가 리포트에 뜨고, 애플리케이션 의존성을 아무리 손봐도 사라지지 않는다.
#
# 버전을 올리는 대신 제거하는 이유: pip 은 빌드 시점 도구이고 audio-worker 는
# 런타임에 임포트하지 않는다(콘솔 스크립트는 pip 이 생성한 평범한 import 문이라
# pkg_resources 를 타지 않는다). 남겨두면 pip 이 벤더를 갱신할 때까지 우리가
# 할 수 있는 일이 없고, 무엇보다 컨테이너 안에서 임의 패키지를 설치할 수 있는
# 능력이 그대로 남는다. 제거는 스캐너 회피가 아니라 그 능력을 없애는 것이다.
#
# setuptools 는 이 이미지에 독립 설치본이 없어(공식 python 이미지는
# --with-ensurepip 만 쓴다) 아래 첫 줄은 보통 no-op 이다. 파이썬 이미지가
# 언젠가 다시 번들하기 시작해도 조용히 통과하도록 남겨둔다.
RUN pip install --no-cache-dir . \
    && pip uninstall --yes setuptools \
    && pip uninstall --yes pip

USER 10001:10001

EXPOSE 9090

ENTRYPOINT ["audio-worker"]
