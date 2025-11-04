#syntax=docker/dockerfile:1.18.0
# builder https://docs.docker.com/build/buildkit/dockerfile-release-notes/
FROM golang:1.25-alpine3.22 AS builder

RUN mkdir /app
COPY . /app
WORKDIR /app


RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build -o seonaut cmd/server/main.go

FROM node:18-alpine3.18 AS front

WORKDIR /home/node
COPY ./web ./app/web


RUN --mount=type=cache,target=/root/.npm \
	npm install --save-exact esbuild && \
	./node_modules/esbuild/bin/esbuild ./app/web/css/style.css \
	--bundle \
	--minify \
	--outdir=./app/web/static \
	--public-path=/resources \
	--loader:.woff=file \
	--loader:.woff2=file

FROM alpine:latest AS production


RUN apk add --no-cache tzdata && \
    ln -snf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone


COPY --from=builder /app/seonaut /app/seonaut
COPY --from=front /home/node/app /app/
COPY ./translations /app/translations
COPY ./migrations /app/migrations
COPY ./config /app/config

ARG TARGETARCH
ARG WAIT_ARCH=${TARGETARCH/amd64/_x86_64}
ARG WAIT_ARCH=${WAIT_ARCH/arm64/_aarch64}
ARG WAIT_ARCH=${WAIT_ARCH/arm_v7/_armv7}
ARG WAIT_ARCH=${WAIT_ARCH:-}
ARG WAIT_VERSION=2.12.1
ADD --chmod=755 https://github.com/ufoscout/docker-compose-wait/releases/download/${WAIT_VERSION}/wait${WAIT_ARCH} /bin/wait

WORKDIR /app