# Start from a Debian image with the latest version of Go installed
# and a workspace (GOPATH) configured at /go.
FROM golang

ARG BUILD_VERSION=development
ARG BUILD_TIME=0

# Copy the local package files to the container's workspace
ADD . /go/src/brokerbot

# Build the brokerbot command inside the container.
RUN go get -v /go/src/brokerbot
RUN go install -ldflags "-X main.buildVersion=$BUILD_VERSION -X main.buildTime=$BUILD_TIME" /go/src/brokerbot

# Run the brokerbot
ENTRYPOINT /go/bin/brokerbot
