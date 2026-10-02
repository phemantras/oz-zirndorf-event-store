# Multi-stage build: a static Go binary on a distroless runtime image without
# shell, running as non-root.

FROM golang:1.27.1 AS build
WORKDIR /src

# Download modules first so this layer stays cached while only code changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/eventstore ./cmd/eventstore

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/eventstore /eventstore
USER nonroot
ENTRYPOINT ["/eventstore"]
