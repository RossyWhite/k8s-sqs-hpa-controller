FROM golang:latest as builder
WORKDIR /tmp/build
COPY . .
RUN GOOS=linux go build .

FROM debian:stretch
RUN mkdir -p /usr/src/app \
  && apt-get update \
  && apt-get install -y ca-certificates \
  && apt-get clean

WORKDIR /usr/src/app
COPY --from=builder /tmp/build/k8s-sqs-hpa-controller ./
ENTRYPOINT ["/usr/src/app/k8s-sqs-hpa-controller"]
