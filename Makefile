
all:
	GOOS=linux go build main.go
	docker build -t pks-cluster-api .
