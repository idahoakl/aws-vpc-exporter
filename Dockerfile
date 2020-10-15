FROM golang:1.14-alpine as builder

ARG SOURCE_COMMIT="N/A"
ARG VERSION="edge"
ARG SOURCE_BRANCH="main"

WORKDIR /go/src/app
COPY . .

RUN CGO_ENABLED=0 go get -d -v ./...
RUN CGO_ENABLED=0 go install \
    -ldflags="-X 'github.com/idahoakl/aws-vpc-exporter/cmd.Revision=${SOURCE_COMMIT}' \
    -X 'github.com/idahoakl/aws-vpc-exporter/cmd.Version=${VERSION}' \
    -X 'github.com/idahoakl/aws-vpc-exporter/cmd.Branch=${SOURCE_BRANCH}'" ./...

FROM alpine:3.12.0
COPY --from=builder /go/bin/aws-vpc-exporter /usr/local/bin/
ENTRYPOINT ["aws-vpc-exporter"]
