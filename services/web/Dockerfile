FROM node:22.13-alpine AS build

WORKDIR /workspace
COPY package.json package-lock.json ./
RUN npm ci
COPY . .

# Vite에는 브라우저에 공개해도 되는 설정만 빌드 시점에 전달한다.
ARG VITE_API_BASE_URL=
ARG VITE_PUBLIC_SITE_URL=
ARG VITE_CDN_BASE_URL=

RUN npm run build

FROM nginxinc/nginx-unprivileged:1.27-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /workspace/dist /usr/share/nginx/html

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
