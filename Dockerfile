# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/job-discovery ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /bin/job-discovery /job-discovery
EXPOSE 8080
ENTRYPOINT ["/job-discovery"]
