all:mixed

mixed:main.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o webhook.amd64.linux main.go
	docker build -t eceasy/kubernetes-webhook:amd64 -f amd64.Dockerfile .
	docker push eceasy/kubernetes-webhook:amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo -o webhook.arm64.linux main.go
	docker build -t eceasy/kubernetes-webhook:arm64 -f arm64.Dockerfile .
	docker push eceasy/kubernetes-webhook:arm64
	docker manifest create eceasy/kubernetes-webhook:1.0.0 eceasy/kubernetes-webhook:amd64 eceasy/kubernetes-webhook:arm64
	docker manifest push --purge eceasy/kubernetes-webhook:1.0.0
	@rm -f webhook.amd64.linux webhook.arm64.linux


x86:main.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o webhook.amd64.linux main.go
	docker build -t eceasy/kubernetes-webhook:amd64 -f amd64.Dockerfile .
	docker push eceasy/kubernetes-webhook:amd64
	@rm -f webhook.amd64.linux

arm64:main.go
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo -o webhook.arm64.linux main.go
	docker build -t eceasy/kubernetes-webhook:arm64 -f arm64.Dockerfile .
	docker push eceasy/kubernetes-webhook:arm64
	@rm -f webhook.arm64.linux

clean:
	@docker images | grep none | awk '{print $3}'| xargs docker rmi
	@rm -f webhook.amd64.linux webhook.arm64.linux
