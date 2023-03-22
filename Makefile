SRHT_PATH?=/usr/lib/python3.10/site-packages/srht
MODULE=buildsrht/
include ${SRHT_PATH}/Makefile

all: api worker

api:
	cd api && go generate ./loaders
	cd api && go generate ./graph
	cd api && go build

worker:
	cd worker && go build

.PHONY: all api worker
