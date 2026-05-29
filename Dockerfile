FROM node:26-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web ./
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -o /out/proxymorph ./cmd/proxymorph

FROM alpine:edge
ENV TZ=Asia/Shanghai
RUN apk add --no-cache ca-certificates sing-box tzdata \
    && cp /usr/share/zoneinfo/$TZ /etc/localtime \
    && echo $TZ > /etc/timezone
WORKDIR /app
COPY --from=build /out/proxymorph /app/proxymorph
VOLUME ["/data"]
EXPOSE 28888
ENTRYPOINT ["/app/proxymorph"]
