ARG GO_VERSION=1.27

FROM golang:${GO_VERSION}-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/quewe ./cmd/quewe

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/quewe /quewe

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/quewe"]