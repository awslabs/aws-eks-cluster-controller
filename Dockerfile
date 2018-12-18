# Build the manager binary
FROM golang:1.10.4 as builder

# Copy in the go src
WORKDIR /go/src/github.com/awslabs/aws-eks-cluster-controller

RUN curl -o /tmp/aws-iam-authenticator --silent --location https://amazon-eks.s3-us-west-2.amazonaws.com/1.10.3/2018-07-26/bin/linux/amd64/aws-iam-authenticator \
    && chmod 0755 /tmp/aws-iam-authenticator \
    && mv /tmp/aws-iam-authenticator /usr/local/bin

COPY vendor/ vendor/
COPY pkg/    pkg/
COPY cmd/    cmd/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager github.com/awslabs/aws-eks-cluster-controller/cmd/manager

# Copy the controller-manager into a thin image
FROM ubuntu:latest
WORKDIR /root/
COPY --from=builder /etc/ssl /etc/ssl
COPY --from=builder /usr/local/bin/aws-iam-authenticator /usr/local/bin/aws-iam-authenticator
COPY --from=builder /go/src/github.com/awslabs/aws-eks-cluster-controller/manager .
ENTRYPOINT ["./manager"]
