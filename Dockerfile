FROM golang:1.17 AS builder

WORKDIR /go/src/github.com/whywaita/nourish

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY . .
RUN go build -o ./app .

FROM chromedp/headless-shell:latest

COPY --from=builder /go/src/github.com/whywaita/nourish/app /app

RUN apt-get update -y \
    && apt-get install -y dumb-init  ca-certificates \
    && apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

ENTRYPOINT ["dumb-init", "--"]
CMD ["/app"]