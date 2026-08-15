# The toolchain version here is a security control, not a preference. Trivy
# reads the Go version stamped into the binary and reports stdlib CVEs against
# it -- 19 of them at 1.24. go.mod only declares the *minimum* language version
# the dependencies need; this line decides what actually ships.
FROM golang:1.26-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/audio-api ./cmd/audio-api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/audio-events ./cmd/audio-events

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/audio-api /audio-api
COPY --from=build /out/audio-events /audio-events

EXPOSE 8080

ENTRYPOINT ["/audio-api"]
