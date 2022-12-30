FROM golang:1.18.9-alpine3.16 as builder

WORKDIR /app/src/proglog

COPY ./ ./
RUN go mod download

RUN CGO_ENABLED=0 go build -o /app/bin/prolog ./cmd/prolog/


FROM scratch
COPY --from=builder /app/bin/prolog /bin/proglog
ENTRYPOINT ["/bin/proglog"]