FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY internal ./internal
COPY services/gateway ./services/gateway
RUN CGO_ENABLED=0 go build -o /out/gateway ./services/gateway

FROM alpine:3.20
RUN adduser -D -H app
USER app
COPY --from=build /out/gateway /gateway
COPY client /client
ENTRYPOINT ["/gateway"]
