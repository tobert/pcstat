FROM golang:1.19.0-alpine3.16 as builder
WORKDIR "/build"
COPY . ./
RUN CGO_ENABLED=0 go build 

FROM alpine:3.15
COPY --from=builder /build/pcstat /usr/local/bin
