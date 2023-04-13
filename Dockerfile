FROM cgr.dev/chainguard/go:latest as builder
WORKDIR /opt
COPY . .
RUN go mod download
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -buildid=" -o ./falco-gpt .

FROM cgr.dev/chainguard/static:latest
WORKDIR /app
COPY --from=builder /opt/falco-gpt /app/falco-gpt
CMD [ "./falco-gpt" ]