FROM node:22.13-alpine AS build

WORKDIR /workspace
COPY package.json package-lock.json ./
RUN npm ci
COPY . .

# Vite에는 브라우저에 공개해도 되는 설정만 빌드 시점에 전달한다.
#
# ⚠️ ARG만 선언하면 Vite에 닿지 않는다. ARG는 Dockerfile 안에서만 보이는
# 값이고, RUN이 실행하는 프로세스의 환경변수로 자동 승격되지 않는다.
# ENV로 옮겨야 `import.meta.env`에 값이 박힌다.
#
# ⚠️ 여기 값은 번들에 그대로 들어가 브라우저에서 읽을 수 있다.
# 비밀 값을 넣지 않는다.
ARG VITE_API_BASE_URL=
ARG VITE_PUBLIC_SITE_URL=
ARG VITE_CDN_BASE_URL=

# 인증이 붙기 전까지 브라우저가 보낼 개발용 subject다. 비어 있으면
# authService가 세션을 만들지 못해 업로드 화면이 차단된다.
ARG VITE_AUTH_MODE=development
ARG VITE_DEV_SUBJECT=

ENV VITE_API_BASE_URL=$VITE_API_BASE_URL \
    VITE_PUBLIC_SITE_URL=$VITE_PUBLIC_SITE_URL \
    VITE_CDN_BASE_URL=$VITE_CDN_BASE_URL \
    VITE_AUTH_MODE=$VITE_AUTH_MODE \
    VITE_DEV_SUBJECT=$VITE_DEV_SUBJECT

RUN npm run build

FROM nginxinc/nginx-unprivileged:1.27-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /workspace/dist /usr/share/nginx/html

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
