FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOWORK=off go build -trimpath -ldflags "-s -w" -o /out/pokemon-global-link .

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 pgl
USER pgl
ENV PGL_HTTP_ADDR=:20160 PGL_LEGALITY_URL=http://pokemon-legality:20170
EXPOSE 20160
COPY --from=build /out/pokemon-global-link /usr/local/bin/pokemon-global-link
ENTRYPOINT ["pokemon-global-link"]
