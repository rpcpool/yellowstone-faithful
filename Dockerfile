FROM ubuntu:22.04 as build

RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y \
        build-essential curl git make wget python3 ca-certificates

ENV GOVER=1.22.3
RUN wget -q https://go.dev/dl/go${GOVER}.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go${GOVER}.linux-amd64.tar.gz && \
    rm go${GOVER}.linux-amd64.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
ENV PATH="/root/.cargo/bin:${PATH}"
RUN /root/.cargo/bin/cargo install cbindgen

WORKDIR /app
COPY . /app

RUN make jsonParsed-linux

RUN for epoch in $(seq 0 786); do \
      bash tools/generate-config-http.sh $epoch;\
    done

FROM ubuntu:22.04 AS runner

WORKDIR /app

COPY --from=build /app/bin/faithful-cli_jsonParsed /app/bin/faithful-cli
COPY --from=build /app/epoch-*.yml /app/tools/

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates && \
    rm -rf /var/lib/apt/lists/*

CMD ["sh", "-c", "./bin/faithful-cli rpc --listen :8180 tools/epoch-*.yml"]

