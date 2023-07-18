FROM alpine:latest

COPY gateway /gateway
COPY internal/fix-acceptor/config/FIX44.xml /FIX44.xml

CMD ["/gateway"]