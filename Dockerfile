FROM alpine:latest AS web-data-handler

RUN apk update
RUN apk upgrade
RUN apk add --update bash go cmake g++ gcc git make vips-dev

COPY --from=golang:1.23-alpine /usr/local/go/ /usr/local/go/
ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /web-data-handler/src

COPY web-data-handler/go.mod web-data-handler/
COPY web-data-handler/go.sum web-data-handler/
COPY core/go.mod                  core/
COPY core/go.sum                  core/
COPY backend/go.mod               backend/
COPY backend/go.sum               backend/
COPY state-consumer/go.mod        state-consumer/
COPY state-consumer/go.sum        state-consumer/

WORKDIR /web-data-handler/src/web-data-handler

RUN go mod download

# include web data handler src
COPY web-data-handler/entries    entries
COPY web-data-handler/migrations migrations
COPY web-data-handler/handler    handler
COPY web-data-handler/main.go    .

# include core src
COPY core/desohash    ../core/desohash
COPY core/consensus   ../core/consensus
COPY core/collections ../core/collections
COPY core/bls         ../core/bls
COPY core/cmd         ../core/cmd
COPY core/lib         ../core/lib
COPY core/migrate     ../core/migrate

# include backend src
COPY backend/apis      ../backend/apis
COPY backend/config    ../backend/config
COPY backend/cmd       ../backend/cmd
COPY backend/miner     ../backend/miner
COPY backend/routes    ../backend/routes
COPY backend/countries ../backend/countries

# include state-consumer src
COPY state-consumer/consumer ../state-consumer/consumer

RUN go mod tidy

## build web data handler backend
RUN GOOS=linux go build -mod=mod -a -installsuffix cgo -o bin/web-data-handler main.go

ENTRYPOINT ["/web-data-handler/src/web-data-handler/bin/web-data-handler"]
