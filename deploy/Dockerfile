# Single parameterized multi-stage build for every binary in cmd/.
# Select which one to build with: --build-arg SERVICE=api|radius|worker|scheduler|migrate
# (Kept DRY as one file instead of Dockerfile.api/.radius/...; the compose file
# passes SERVICE per service.)
#
# Builds offline from the committed vendor/ directory (hermetic, no network),
# and uses the base image's toolchain (GOTOOLCHAIN=local) so no toolchain is
# downloaded at build time.

FROM golang:1.25-alpine AS build
ARG SERVICE=api
ENV GOTOOLCHAIN=local
WORKDIR /src

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -trimpath -ldflags="-s -w" \
    -o /out/app ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
