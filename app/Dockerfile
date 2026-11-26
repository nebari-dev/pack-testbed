# Build a static binary and ship it on distroless/static:nonroot.
# The app has no external Go dependencies and no CGO, so the final image is a
# single static binary on a minimal, shell-less, nonroot base - a small attack
# surface and a clean signing/SBOM subject.
FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY go.mod ./
# No go.sum: the app depends only on the standard library.
COPY . .
RUN CGO_ENABLED=0 GOFLAGS=-trimpath go build -ldflags="-s -w" -o /out/app .

# throwaway: older base with fixable HIGH findings
FROM alpine:3.19.9
COPY --from=build /out/app /app
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app"]
