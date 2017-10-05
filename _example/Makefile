.PHONY: get-deps up all

all: up

get-deps:
	cpanm Carton
	carton install

up: get-deps
	carton exec proclet start
