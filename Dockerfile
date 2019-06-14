FROM golang
COPY . /root/
WORKDIR /root
RUN go install

RUN chmod +x entrypoint.sh
ENTRYPOINT ["/root/entrypoint.sh"]