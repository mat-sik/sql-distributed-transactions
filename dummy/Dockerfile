FROM golang:1.24.4 AS build-stage

RUN git config --global url.ssh://git@github.com/.insteadOf https://github.com/
RUN mkdir /root/.ssh && ssh-keyscan github.com >> /root/.ssh/known_hosts
ENV GOPRIVATE=github.com/mat-sik

WORKDIR /app
COPY go.mod go.sum ./
COPY /cmd ./cmd
COPY /internal ./internal

RUN --mount=type=secret,id=dockerBuildKey,dst=/root/.ssh/id_rsa go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -C ./cmd -o main

FROM alpine:latest AS build-release-stage
COPY --from=build-stage app/cmd/main /main

EXPOSE 40690
CMD ["/main"]