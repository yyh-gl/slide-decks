FROM golang:1.16.0-alpine

WORKDIR /go/src/github.com/yyh-gl/slide-decks

RUN go get golang.org/x/tools/cmd/present

EXPOSE 3999

CMD ["present", "-http", "0.0.0.0:3999", "-orighost", "localhost"]
