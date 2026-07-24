# Built by GoReleaser's dockers_v2 pipe, which stages each platform's binary
# under <goos>/<goarch>/ in the build context — hence the $TARGETPLATFORM path.
FROM scratch
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/kdbx /kdbx
ENTRYPOINT ["/kdbx"]
