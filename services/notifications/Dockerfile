FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY internal ./internal
COPY services/notifications ./services/notifications
RUN CGO_ENABLED=0 go build -o /out/notifications ./services/notifications

FROM alpine:3.20
RUN adduser -D -H app
USER app
COPY --from=build /out/notifications /notifications
ENTRYPOINT ["/notifications"]
