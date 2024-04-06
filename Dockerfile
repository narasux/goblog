# -------------- builder container --------------
FROM golang:1.22 as builder

WORKDIR /go/src/

COPY . .

RUN go env -w GO111MODULE=on
RUN go env -w GOPROXY=https://goproxy.cn,direct

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o goblog ./main.go

# -------------- runner container --------------
FROM alpine:3.19 AS runner

RUN apk --update --no-cache add bash

WORKDIR /data/workspace

COPY --from=builder /go/src/goblog /usr/bin/goblog

COPY --from=builder /go/src/data /data/workspace/data

ENV BLOG_DATA_BASE_DIR=/data/workspace/data

COPY --from=builder /go/src/templates /data/workspace/templates

ENV TMPL_FILE_BASE_DIR=/data/workspace/templates

# TODO 后续图片数量增加后，可能导致镜像体积增大，可以考虑使用 COS ？
COPY --from=builder /go/src/static /data/workspace/static

ENV STATIC_FILE_BASE_DIR=/data/workspace/static

RUN mkdir -p /data/logs/

ENV LOG_FILE_BASE_DIR=/data/logs

ENTRYPOINT ["goblog", "webserver"]