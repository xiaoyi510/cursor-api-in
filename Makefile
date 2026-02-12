IMAGE := registry.cn-chengdu.aliyuncs.com/xarr/xarr-cursor-api-2-claude
TAG := latest

.PHONY: build push run clean fast-build fast-push

build:
	docker build --platform linux/amd64 -t $(IMAGE):$(TAG) .

push: build
	docker push $(IMAGE):$(TAG)

fast-build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server .
	docker build --platform linux/amd64 -f Dockerfile.fast -t $(IMAGE):$(TAG) .

fast-push: fast-build
	docker push $(IMAGE):$(TAG)

run:
	docker compose up -d

clean:
	docker compose down
	docker rmi $(IMAGE):$(TAG) 2>/dev/null || true
	rm -f server
