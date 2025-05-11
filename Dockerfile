FROM golang:1.24 AS base


# Build statically compiled binary
FROM base AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/centralwebhook


# Run the tests in the container
FROM build AS run-test-stage
RUN go test -v ./...


# Add a non-root user to passwd file
FROM base AS useradd
RUN groupadd -g 1000 nonroot
RUN useradd -u 1000 nonroot -g 1000


# Deploy the application binary into sratch image
FROM scratch AS release
WORKDIR /app
COPY --from=build /app/centralwebhook /app/centralwebhook
COPY --from=useradd /etc/group /etc/group
COPY --from=useradd /etc/passwd /etc/passwd
USER nonroot:nonroot
ENTRYPOINT ["/app/centralwebhook"]
