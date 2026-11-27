# Build a static binary and ship it on distroless/static:nonroot.
# The app has no external Go dependencies and no CGO, so the final image is a
# single static binary on a minimal, shell-less, nonroot base - a small attack
# surface and a clean signing/SBOM subject.
FROM golang:1.26.10-bookworm@sha256:ee3901c61e423732605a51f1b8157c849ec01194ea0a274213b5202b2c11e7bf AS build
WORKDIR /src
COPY go.mod ./
# No go.sum: the app depends only on the standard library.
COPY . .
RUN CGO_ENABLED=0 GOFLAGS=-trimpath go build -ldflags="-s -w" -o /out/app .

FROM gcr.io/distroless/static:nonroot@sha256:8ce6dd87e9823c5b48dcfb2c8604714ccb351ce59607327bcc4957448da279b5
COPY --from=build /out/app /app
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app"]
