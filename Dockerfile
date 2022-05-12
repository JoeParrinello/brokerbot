FROM golang:1.18

WORKDIR $GOPATH/src/brokerbot

# pre-copy/cache go.mod for pre-downloading dependencies and only redownload them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

ARG BUILD_VERSION=development
ARG BUILD_TIME=0

COPY . .
RUN go build -v  -ldflags "-X main.buildVersion=$BUILD_VERSION -X main.buildTime=$BUILD_TIME" -o $GOPATH/bin/brokerbot .

CMD ["brokerbot"]
