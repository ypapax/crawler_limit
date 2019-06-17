ARG GO_VERSION=1.11
FROM golang:${GO_VERSION}
COPY . /root/
WORKDIR /root
RUN go install

RUN chmod +x entrypoint.sh
ENTRYPOINT ["/root/entrypoint.sh"]