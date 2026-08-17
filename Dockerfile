# syntax=docker/dockerfile:1

# ---- build stage: pinned Go 1.26, never "latest" ----
FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS builder
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
ARG TARGETOS=linux
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/eurobatt ./cmd/eurobatt

# ---- runtime stage: only the executable ----
FROM scratch
COPY --from=builder /out/eurobatt /eurobatt
ENTRYPOINT ["/eurobatt"]
CMD ["help"]
