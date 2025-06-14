.PHONY: run build clean

run:
	go run cmd/warofages/main.go

build:
	go build -o ./bin/warofages cmd/warofages/main.go

clean:
	rm ./bin/warofages && rm -d ./bin/

updateAll: updateRepo updateSubmodule

updateRepo:
	git pull

updateSubmodule:
	git submodule update --recursive --remote --init

full: updateAll build
	./bin/warofages
