# Build statically compiled binary
FROM golang:1.23 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /odkhook


# Run the tests in the container
FROM build AS run-test-stage
RUN go test -v ./...


# Deploy the application binary into sratch image
FROM scratch as release
WORKDIR /
COPY --from=build /odkhook /odkhook
USER nonroot:nonroot
ENTRYPOINT ["/odkhook"]
