FROM golang:alpine as builder
MAINTAINER Jessica Frazelle <jess@linux.com>

ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go

RUN	apk add --no-cache \
	ca-certificates

COPY . /go/src/github.com/genuinetools/netns

RUN set -x \
	&& apk add --no-cache --virtual .build-deps \
		git \
		gcc \
		libc-dev \
		linux-headers
		libgcc \
		make \
	&& cd /go/src/github.com/genuinetools/netns \
	&& make static \
	&& mv netns /usr/bin/netns \
	&& apk del .build-deps \
	&& rm -rf /go \
	&& echo "Build complete."

FROM alpine:latest

COPY --from=builder /usr/bin/netns /usr/bin/netns
COPY --from=builder /etc/ssl/certs/ /etc/ssl/certs

ENTRYPOINT [ "netns" ]
CMD [ "--help" ]
