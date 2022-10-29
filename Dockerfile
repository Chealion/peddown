FROM golang:1.19.2-alpine
ENV CGO_ENABLED=1

RUN apk update && apk upgrade
RUN apk add --no-cache sqlite build-base

WORKDIR /

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Install golang app
COPY *.go ./
COPY database /usr/local/go/src/database

RUN go build -o peddown tweetCollector.go
RUN chmod a+x ./peddown

# Volume is mounted in fly.toml but fails if mentioned here?
RUN ln -s /data/peddown.sqlite /peddown.db
CMD [ "/peddown" ]