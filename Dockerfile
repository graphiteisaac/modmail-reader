FROM golang:1.20 as base

WORKDIR /app
### standard lib only :3
#COPY go.mod go.sum ./
#RUN go mod download
COPY go.mod ./

COPY *.go ./
COPY view ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /app

FROM gcr.io/distroless/static-debian11
COPY --from=base /app .

EXPOSE 8087
CMD ["/app", "--app-port", "8087"]