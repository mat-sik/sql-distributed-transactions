FROM golang:1.24.4 AS build-stage

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -C ./cmd -o main

FROM gcr.io/distroless/base-debian12 AS build-release-stage
COPY --from=build-stage app/cmd/main /main

CMD ["/main"]