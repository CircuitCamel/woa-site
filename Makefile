.PHONY: run build clean

run:
	go run cmd/warofages/main.go

build:
	go build -o ./warofages cmd/warofages/main.go

clean:
	rm ./warofages