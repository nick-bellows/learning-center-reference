# Multi-stage build: compile in a full Go image, then copy just the binary into a tiny,
# secure base. The final image contains no shell, package manager, or OS cruft.

# --- build stage ---
FROM golang:1.26-alpine AS build
WORKDIR /src
# Copy the module files first and download deps. This layer is cached unless go.mod/go.sum
# change, so rebuilds are fast when only source changes.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CGO disabled + static build so the binary runs on the minimal base below.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/resetdemo ./cmd/resetdemo

# --- run stage ---
FROM gcr.io/distroless/static-debian12:nonroot AS run
COPY --from=build /out/server /server
COPY --from=build /out/resetdemo /resetdemo
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]
