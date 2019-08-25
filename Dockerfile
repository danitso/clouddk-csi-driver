#==================================================== BUILD STAGE ====================================================#
FROM golang:1.12-alpine AS build

ENV BUILD_UTILITIES="git make"

# Install the build utilities.
RUN apk add --no-cache ${BUILD_UTILITIES}

# Build the software.
COPY . /build
WORKDIR /build
RUN make build
#==================================================== FINAL STAGE ====================================================#
FROM alpine:latest
#==================================================== INFORMATION ====================================================#
LABEL Description="Kubernetes CSI Driver for Cloud.dk"
LABEL Maintainer="Danitso <info@danitso.com>"
LABEL Vendor="Danitso"
#==================================================== INFORMATION ====================================================#
ENV LANG="C.UTF-8"
ENV REQUIRED_PACKAGES="ca-certificates nfs-utils"

# Install the required packages.
RUN apk add --no-cache ${REQUIRED_PACKAGES}

# Copy the binary from the build stage.
COPY --from=build /build/bin/clouddk-csi-driver /usr/bin/clouddk-csi-driver

# Ensure that the binary can be executed.
RUN chmod +x /usr/bin/clouddk-csi-driver

# Set the entrypoint as we will not be requiring shell access.
ENTRYPOINT [ "/usr/bin/clouddk-csi-driver" ]
