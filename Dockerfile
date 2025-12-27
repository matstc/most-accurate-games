# build
FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN ls -la /app
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app

# run
FROM alpine:latest
WORKDIR /app
COPY --from=build /app /app
EXPOSE 8080
CMD ["./app"]
