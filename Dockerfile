FROM golang:1.20.2-alpine as builder
WORKDIR /opt
COPY . .
RUN go mod download
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ./falco-gpt .

FROM alpine:3.17
WORKDIR /app
COPY --from=builder /opt/falco-gpt /app/falco-gpt
CMD [ "./falco-gpt" ]