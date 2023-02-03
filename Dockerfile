FROM golang:1.18.9-alpine3.16 as builder

WORKDIR /app/src/prolog

COPY ./ ./
RUN go mod download

RUN CGO_ENABLED=0 go build -o /app/bin/prolog ./cmd/prolog/


FROM alpine

COPY --from=builder /app/bin/prolog /bin/prolog

#ENTRYPOINT ["/bin/prolog"]