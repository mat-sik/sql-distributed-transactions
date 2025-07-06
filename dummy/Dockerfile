FROM golang:1.24.4 AS build-stage

WORKDIR /app
COPY go.mod go.sum ./
COPY /cmd ./cmd
COPY /internal ./internal

RUN go mod download && go mod verify
RUN CGO_ENABLED=0 GOOS=linux go build -C ./cmd -o main

FROM alpine:latest AS build-release-stage
COPY --from=build-stage app/cmd/main /main

EXPOSE 40690
CMD ["/main"]