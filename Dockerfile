FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/app ./cmd/app \
	&& CGO_ENABLED=0 GOOS=linux go build -o /out/migrate ./cmd/migrate

FROM alpine:3.22

WORKDIR /app
RUN adduser -D -H appuser

COPY --from=build /out/app /app/app
COPY --from=build /out/migrate /app/migrate
COPY migrations /app/migrations
COPY templates /app/templates
COPY static /app/static

RUN mkdir -p /data/file-storage && chown -R appuser:appuser /data /app
USER appuser

EXPOSE 8080
CMD ["/app/app"]
