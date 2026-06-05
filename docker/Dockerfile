FROM golang:alpine

RUN apk update && apk upgrade
RUN apk add --no-cache sqlite build-base tzdata

WORKDIR /

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Install golang app
COPY *.go ./

RUN go build -o peddown .
RUN chmod a+x ./peddown

# Create a symlink so the container can access the database stored on a volume
RUN ln -s /data/incidents.db /incidents.db

COPY docker/entrypoint.sh .
RUN chmod a+x ./entrypoint.sh

ENTRYPOINT ["./entrypoint.sh"]
