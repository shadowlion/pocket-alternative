.IPHONY: run build

run:
	@go run . serve

build:
	@go build -o main .
