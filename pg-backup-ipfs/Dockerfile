FROM golang:1.20 as build_pg-backup-ipfs
ENV CGO_ENABLED 0
ENV GO111MODULE=on

# Create the service directory and the copy the module files first and then
# download the dependencies. If this doesn't change, we won't need to do this
# again in future builds.
RUN mkdir /service
COPY go.* /service/
WORKDIR /service
RUN go mod download

# Copy the source code into the container.
COPY . /service

# Build the service binary.
WORKDIR /service
RUN go build


# Run the Go Binary in Alpine.
FROM alpine:3.17
RUN addgroup -g 1000 -S agnt && \
    adduser -u 1000 -h /service -G agnt -S agnt
COPY --from=build_pg-backup-ipfs --chown=agnt:agnt /service/pg-backup-ipfs /service/pg-backup-ipfs
WORKDIR /service
USER agnt
CMD ["./pg-backup-ipfs"]