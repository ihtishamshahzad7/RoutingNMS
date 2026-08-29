FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/routingnms-api ./cmd/api

FROM alpine:3.22
RUN addgroup -S routingnms && adduser -S -G routingnms routingnms
COPY --from=build /out/routingnms-api /usr/local/bin/routingnms-api
USER routingnms
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/routingnms-api"]
