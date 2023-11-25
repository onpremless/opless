FROM golang:alpine AS bootstrap

WORKDIR /build
RUN apk add --no-cache git
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .

RUN go build -o /manager

# - - - - - - - - - - # 

FROM alpine AS lambda

WORKDIR /
COPY --from=bootstrap /manager /manager

ENTRYPOINT ["/manager"]
