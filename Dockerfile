# syntax=docker/dockerfile:1

FROM golang:1.26.11-bookworm AS build
WORKDIR /build
COPY go.mod *.go ./
COPY web ./web
COPY assets/brand ./assets/brand
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/subsoil .

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/subsoil /usr/local/bin/subsoil
COPY LICENSE /usr/share/licenses/subsoil/LICENSE
EXPOSE 5000
ENTRYPOINT ["/usr/local/bin/subsoil"]
CMD ["serve", "--config", "/etc/subsoil/config.json", "--listen", "0.0.0.0:5000"]
