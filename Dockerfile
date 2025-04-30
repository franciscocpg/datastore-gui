FROM --platform=$BUILDPLATFORM golang:1.24 AS go
ARG TARGETOS
ARG TARGETARCH
WORKDIR /go/src/app
COPY . /go/src/app
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /go/bin/app

FROM node:20-slim
COPY --from=go /go/bin/app /
COPY ./entrypoint.sh /
COPY ./client /client
RUN cd /client && yarn install && chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
