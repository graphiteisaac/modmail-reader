FROM golang:1.20 as base

WORKDIR /app
### standard lib only :3
#COPY go.mod go.sum ./
#RUN go mod download
COPY go.mod ./

COPY *.go ./
COPY static ./static
COPY view ./view
RUN CGO_ENABLED=0 GOOS=linux go build -o main

FROM gcr.io/distroless/static-debian11
COPY --from=base /app/main .
COPY --from=base /app/view ./view

EXPOSE 8087
CMD ["/main", "-host-port", "8087"]
