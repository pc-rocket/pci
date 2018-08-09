FROM golang:1.10.3

RUN apt-get update
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
ADD . /go/src/github.com/pc-rocket/pci
WORKDIR /go/src/github.com/pc-rocket/pci
RUN dep ensure
RUN go install .