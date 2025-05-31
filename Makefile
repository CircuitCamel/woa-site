.PHONY: run build clean

run:
	go run cmd/warofages/main.go

build:
	go build -o ./WarOfAges cmd/warofages/main.go

clean:
	rm ./WarOfAges