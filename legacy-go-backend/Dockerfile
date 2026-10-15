# Build on the runner's own CPU and cross-compile for the target: fast multi-arch images, no emulation.
FROM --platform=$BUILDPLATFORM golang:1.26 AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY backend/go.mod ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/devoker ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/devoker /devoker
EXPOSE 8080
# Numeric, so Kubernetes runAsNonRoot can verify it (65532 is distroless "nonroot").
USER 65532:65532
ENTRYPOINT ["/devoker"]
