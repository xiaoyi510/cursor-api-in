IMAGE := registry.cn-chengdu.aliyuncs.com/xarr/xarr-cursor-api-2-claude
TAG := latest

.PHONY: build push run clean

build:
	docker build --platform linux/amd64 -t $(IMAGE):$(TAG) .

push: build
	docker push $(IMAGE):$(TAG)

run:
	docker compose up -d

clean:
	docker compose down
	docker rmi $(IMAGE):$(TAG) 2>/dev/null || true
