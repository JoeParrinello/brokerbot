# Start from a Debian image with the latest version of Go installed
# and a workspace (GOPATH) configured at /go.
FROM golang

ARG BUILD_VERSION=development
ARG BUILD_TIME=0

# Copy the local package files to the container's workspace
ADD . /go/src/hardorange/brokerbot

# Build the brokerbot command inside the container.
RUN go get /go/src/hardorange/brokerbot
RUN go install -ldflags "-X main.buildVersion=$BUILD_VERSION -X main.buildTime=$BUILD_TIME" /go/src/hardorange/brokerbot

# Run the brokerbot
ENTRYPOINT /go/bin/brokerbot