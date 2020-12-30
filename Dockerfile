# Start from a Debian image with the latest version of Go installed
# and a workspace (GOPATH) configured at /go.
FROM golang

# Copy the local package files to the container's workspace
ADD . /go/src/hardorange/brokerbot


# Build the brokerbot command inside the container.
RUN go install /go/src/hardorange/brokerbot

# Run the brokerbot
ENTRYPOINT /go/bin/brokerbot