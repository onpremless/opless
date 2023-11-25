FROM golang:alpine AS bootstrap
ARG PROJECT="UNKNOWN"

WORKDIR /build
RUN apk add --no-cache git
COPY . .

RUN go build -o /${PROJECT} ./${PROJECT}/main.go

# - - - - - - - - - - # 

FROM alpine AS lambda
ARG PROJECT="UNKNOWN"
WORKDIR /
COPY --from=bootstrap /${PROJECT} /svc

ENTRYPOINT ["/svc"]