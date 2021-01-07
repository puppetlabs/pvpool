FROM golang:1.14-alpine AS builder
ENV CGO_ENABLED 0
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN go build -a -installsuffix cgo -o /go/bin/pvpool-controller

FROM gcr.io/distroless/static
COPY --from=builder /go/bin/pvpool-controller /bin/pvpool-controller
CMD ["/bin/pvpool-controller"]

