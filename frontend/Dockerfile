FROM node:22-alpine AS deps
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci

FROM node:22-alpine AS builder
WORKDIR /src
COPY --from=deps /src/node_modules ./node_modules
COPY . .
ENV NEXT_TELEMETRY_DISABLED=1
# 构建时注入 API 地址（生产部署通过 Nginx 反代时使用 /api/v1）
ARG NEXT_PUBLIC_API_BASE_URL=/api/v1
ARG API_INTERNAL_BASE_URL=http://api:8080/api/v1
ENV NEXT_PUBLIC_API_BASE_URL=${NEXT_PUBLIC_API_BASE_URL}
ENV API_INTERNAL_BASE_URL=${API_INTERNAL_BASE_URL}
RUN mkdir -p /src/public && npm run build

FROM node:22-alpine AS runner
WORKDIR /src
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=builder /src/.next/standalone ./
COPY --from=builder /src/.next/static ./.next/static
COPY --from=builder /src/public ./public
EXPOSE 3000
CMD ["node", "server.js"]
