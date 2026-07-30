# Built by GoReleaser's dockers_v2 pipe, which stages each platform's binary
# under <goos>/<goarch>/ in the build context — hence the $TARGETPLATFORM path.
# OCI labels (image.source and friends) are set from .goreleaser.yaml, where the
# version and revision can be templated from the release tag.
FROM scratch
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/kdbx /kdbx
ENTRYPOINT ["/kdbx"]
