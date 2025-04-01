FROM golang:1.24 AS go
WORKDIR /go/src/app
COPY . /go/src/app
RUN go build -o /go/bin/app

FROM node:20-slim
COPY --from=go /go/bin/app /
COPY ./entrypoint.sh /
COPY ./client /client
RUN cd /client && yarn install && chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
