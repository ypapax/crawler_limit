FROM golang
COPY . /root/
WORKDIR /root
RUN go install

RUN chmod +x entrypoint.sh
RUN ls -la
ENTRYPOINT ["/root/entrypoint.sh"]
#ENTRYPOINT crawler_limit  -v 4 -log_dir /tmp -url $@