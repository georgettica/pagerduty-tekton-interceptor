ARG  RUNNER_IMAGE=redhat/ubi8-minimal@sha256:9a81cce19ae2a962269d4a7fecd38aec60b852118ad798a265c3f6c4be0df610
FROM golang@sha256:3afd220509acf9866e91932a3a41bf341b8bada82107ef3ecce3422826b98064 as builder

RUN apk add --update --no-cache ca-certificates tzdata git make bash && update-ca-certificates

ADD . /opt
WORKDIR /opt

RUN go install github.com/go-task/task/v3/cmd/task@latest

# CGO_ENABLED is set to allow the golang binary build to work on the ubi image
RUN pwd; find
RUN git update-index --refresh; ls -lah ; CGO_ENABLED=0 ${GOPATH}/bin/task


FROM ${RUNNER_IMAGE} as runner

COPY --from=builder /opt/obnpctl /bin/obnpctl

ARG BUILD_DATE
ARG VERSION
ARG VCS_REF
ARG DOCKERFILE_PATH

LABEL vendor="Ron Green" \
    name="geoegettica/obnpctl" \
    description="a CLI tool explain basic k8s networking" \
    io.k8s.display-name="geoegettica/obnpctl" \
    io.k8s.description="a CLI tool explain basic k8s networking" \
    maintainer="Ron Green <8326+rogreen@users.noreply.gitlab.cee.redhat.com>" \
    version="$VERSION" \
    org.label-schema.build-date=$BUILD_DATE \
    org.label-schema.description="a CLI tool explain basic k8s networking" \
    org.label-schema.docker.cmd="docker run --rm  geoegettica/obnpctl" \
    org.label-schema.docker.dockerfile=$DOCKERFILE_PATH \
    org.label-schema.name="geoegettica/obnpctl" \
    org.label-schema.schema-version="0.1.0" \
    org.label-schema.vcs-branch=$VCS_BRANCH \
    org.label-schema.vcs-ref=$VCS_REF \
    org.label-schema.vcs-url="https://github.com:georgettica/obnpctl" \
    org.label-schema.vendor="geoegettica/obnpctl" \
    org.label-schema.version=$VERSION

EXPOSE 8080
ENTRYPOINT ["/bin/pagerduty-tekton-interceptor"]
