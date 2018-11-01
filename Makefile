default: docker

docker:
	GOOS=linux $(MAKE) build
	docker build -t pks-cluster-api .

certs:
ifeq ("$(wildcard server.crt)","")
	openssl genrsa -des3 -passout pass:x -out server.pass.key 2048
	openssl rsa -passin pass:x -in server.pass.key -out server.key
	rm server.pass.key
	openssl req -new -key server.key -out server.csr -subj '/CN=pks.example.com/O=My Company Name LTD./C=US'
	openssl x509 -req -sha256 -days 365 -in server.csr -signkey server.key -out server.crt
	rm server.csr
endif

run: certs
	go run main.go

build:
	go build .

deploy:
	kubectl apply -f deployment.yaml

clean:
	rm -f server.crt server.key
