FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/tracko ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates wget

WORKDIR /app
COPY --from=build /out/tracko /usr/local/bin/tracko

USER nobody

ENTRYPOINT ["/usr/local/bin/tracko"]
