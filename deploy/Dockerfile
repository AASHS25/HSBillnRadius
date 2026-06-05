# Single parameterized multi-stage build for every binary in cmd/.
# Select which one to build with: --build-arg SERVICE=api|radius|worker|scheduler|migrate
# (Kept DRY as one file instead of Dockerfile.api/.radius/...; the compose file
# passes SERVICE per service.)

FROM golang:1.25-alpine AS build
ARG SERVICE=api
WORKDIR /src

# Cache module downloads separately from the source tree.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/app ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
