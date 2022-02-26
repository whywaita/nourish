FROM golang:1.17 AS builder

WORKDIR /go/src/github.com/whywaita/nourish

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

COPY . .
RUN go build -o ./app .

FROM alpine

RUN apk update \
  && apk update
RUN apk add --no-cache ca-certificates \
  && update-ca-certificates 2>/dev/null || true

COPY --from=builder /go/src/github.com/whywaita/nourish/app /app

CMD ["/app"]