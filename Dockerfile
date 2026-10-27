# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/openfanta ./cmd/openfanta

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/openfanta /usr/local/bin/openfanta
USER nonroot:nonroot
EXPOSE 8080
ENV OPENFANTA_HTTP_ADDR=:8080
ENTRYPOINT ["/usr/local/bin/openfanta"]
CMD ["serve"]
