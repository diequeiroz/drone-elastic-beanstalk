FROM golang:1.9-alpine
WORKDIR /go/src/github.com/quintoandar/drone-elasticbeanstalk
ADD . .
RUN GOOS=linux CGO_ENABLED=0 go build -o /bin/drone-elasticbeanstalk \
    github.com/quintoandar/drone-elasticbeanstalk

FROM scratch
COPY --from=0 /bin/drone-elasticbeanstalk /bin/drone-elasticbeanstalk
ENTRYPOINT ["/bin/drone-elasticbeanstalk"]
