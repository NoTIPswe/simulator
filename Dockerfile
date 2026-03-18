# Builder
FROM ghcr.io/notipswe/notip-angular-base:v0.0.1 AS builder

USER node
WORKDIR /app
COPY --chown=node:node package*.json ./
RUN npm ci

COPY --chown=node:node . .

RUN npm run build -- --configuration production --output-path=dist-generic


# Production
FROM nginxinc/nginx-unprivileged:1-alpine AS prod

USER root
WORKDIR /app

COPY --chown=nginx:nginx --from=builder /app/dist-generic/browser /usr/share/nginx/html

USER nginx
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD curl -f http://localhost:8080/ || exit 1

CMD ["nginx", "-g", "daemon off;"]
