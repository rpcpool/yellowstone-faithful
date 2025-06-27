CURRENT_COMMIT := $(shell git rev-parse --short HEAD)

build:
	docker buildx build --platform linux/amd64,linux/arm64 -t ghcr.io/notwedtm/faithful-cli:$(CURRENT_COMMIT) --push --dockerfile deploy/faithful-cli.Dockerfile .

deploy:
	helm upgrade --install car-processing --namespace car-processor --create-namespace chart --set image.tag=$(CURRENT_COMMIT)

delete:
	helm delete car-processing --namespace car-processor

bundle:
	git add -A
	git commit -m "Update"
	make build
	make reset
	sleep 10
	make deploy
