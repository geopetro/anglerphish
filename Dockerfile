# Minify client side assets (JavaScript)
FROM node:22 AS build-js

RUN npm install gulp gulp-cli -g

WORKDIR /build
COPY . .
RUN npm install --only=dev
RUN gulp


# Build Golang binary
FROM golang:1.24 AS build-golang

WORKDIR /go/src/github.com/gophish/gophish
COPY . .
RUN go get -v && go build -v


# Runtime container
FROM debian:stable-slim

RUN useradd -m -d /opt/gophish -s /bin/bash app

RUN apt-get update && \
	apt-get install --no-install-recommends -y jq libcap2-bin ca-certificates python3 python3-venv && \
	apt-get clean && \
	rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /opt/gophish
COPY --chown=app:app --from=build-golang /go/src/github.com/gophish/gophish/gophish ./
COPY --chown=app:app --from=build-js /build/static/ ./static/
COPY --chown=app:app db/ ./db/
COPY --chown=app:app templates/ ./templates/
COPY --chown=app:app docker/ ./docker/
COPY --chown=app:app reports/python/requirements.txt ./reports/python/
COPY --chown=app:app config.json VERSION ANGLERPHISH_VERSION LICENSE README.md ./

RUN setcap 'cap_net_bind_service=+ep' /opt/gophish/gophish

USER app
RUN mkdir -p reports/files && \
	python3 -m venv reports/python/venv && \
	reports/python/venv/bin/pip install --no-cache-dir -r reports/python/requirements.txt
RUN sed -i 's/127.0.0.1/0.0.0.0/g' config.json
RUN touch config.json.tmp

EXPOSE 3333 8080 8443 80

CMD ["./docker/run.sh"]
