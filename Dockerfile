# syntax=docker/dockerfile:1

# --- build stage -----------------------------------------------------------
FROM golang:1.23-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=docker
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/workgraph ./cmd/workgraph

# --- runtime stage ---------------------------------------------------------
# alpine (not scratch/distroless) because workgraph shells out to the git CLI
# for its sync/commit/push preflight.
FROM alpine:3.20
RUN apk add --no-cache git ca-certificates \
 && git config --system --add safe.directory '*'   # tolerate mounted repos owned by the host user
COPY --from=build /out/workgraph /usr/local/bin/workgraph
WORKDIR /workspace
ENTRYPOINT ["workgraph"]
CMD ["--help"]
