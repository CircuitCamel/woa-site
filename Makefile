.PHONY: run build clean

run:
	git submodule update --init && go run cmd/warofages/main.go

build:
	git submodule update --init && go build -o ./bin/warofages cmd/warofages/main.go

clean:
	rm ./bin/warofages && rm -d ./bin/

full: build
	./bin/warofages