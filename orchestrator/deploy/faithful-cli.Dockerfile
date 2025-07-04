FROM golang:1.22-alpine AS builder

# Install git and other build dependencies
RUN apk add --no-cache git make

# Clone the Yellowstone Faithful repository
WORKDIR /src
RUN git clone --depth 1 https://github.com/rpcpool/yellowstone-faithful.git

# Build the faithful-cli
WORKDIR /src/yellowstone-faithful
RUN go build -o /faithful-cli

# Create a minimal runtime image
FROM alpine:3.19
COPY --from=builder /faithful-cli /usr/local/bin/faithful-cli

# Install aria2
RUN apk add --no-cache aria2 bash redis

# Install tar
RUN apk add --no-cache tar jq

# Install s4cmd (prefer system package, fallback to venv if not available)
RUN apk add --no-cache py3-s4cmd || (apk add --no-cache python3 py3-pip py3-setuptools py3-wheel && python3 -m venv /venv && /venv/bin/pip install s4cmd && ln -s /venv/bin/s4cmd /usr/local/bin/s4cmd)

COPY scripts/process.sh scripts/run.sh /
RUN chmod +x /process.sh /run.sh

# Set default command
ENTRYPOINT ["/run.sh"]
