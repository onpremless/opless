FROM golang:alpine AS bootstrap

WORKDIR /build
RUN apk add git
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .

RUN go build -o /handler

# - - - - - - - - - - # 

FROM alpine AS handler

WORKDIR /
COPY --from=bootstrap /handler /handler

ENTRYPOINT ["/handler"]
