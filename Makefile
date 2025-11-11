BINARY_NAME=bot-go
EVAL_BINARY_NAME=run_eval
DOCKER_IMAGE=bot
VERSION ?= latest
REGISTRY ?= armchr
MAIN_PATH=./cmd/main.go
EVAL_PATH=./cmd/run_eval.go
VENV_DIR=.venv

.PHONY: build build-eval run run-eval clean test deps install-lsp-servers setup-python-env docker-build docker-run docker-run-detached docker-run-with-workdir docker-stop docker-logs docker-compose-up docker-compose-down docker-push docker-tag

build:
	go build -o bin/$(BINARY_NAME) $(MAIN_PATH)

build-eval:
	go build -o bin/$(EVAL_BINARY_NAME) $(EVAL_PATH)

run:
	go run $(MAIN_PATH) -source=config/source.yaml -app=config/app.yaml

run-eval:
	@if [ -z "$(TEST)" ]; then \
		echo "Usage: make run-eval TEST=eval/test_case_01_lsp_client_init OUTPUT=results.json"; \
		echo "   or: make run-eval TEST=eval OUTPUT=all_results.json"; \
		exit 1; \
	fi
	@OUTPUT_FILE=$${OUTPUT:-eval_results.json}; \
	bin/$(EVAL_BINARY_NAME) -test=$(TEST) -output=$$OUTPUT_FILE -app=config/app.yaml -source=config/source.yaml

run_test:
	go run $(MAIN_PATH) -config=source.yaml -test

clean:
	go clean
	rm -f bin/$(BINARY_NAME)
	rm -f bin/$(EVAL_BINARY_NAME)
	rm -f *.log
	rm -f cmd/*.log
	rm -f eval_results.json

test:
	go test ./...

deps:
	go mod download
	go mod tidy

# Docker targets for single image
docker-build:
	docker build -t $(DOCKER_IMAGE):$(VERSION) .

docker-run:
	docker run -it --rm \
	--ulimit nofile=65536:65536 \
	-p 8181:8181 \
	-p 8282:8282 \
	-v $(PWD)/config/source.yaml:/app/config/source.yaml:ro \
	-v $(PWD)/config/app.yaml:/app/config/app.yaml:ro \
	-v $(PWD)/data:/app/data \
	-v $(PWD)/logs:/app/logs \
	--name bot-go \
	$(DOCKER_IMAGE):$(VERSION) \
	./bot-go -app=config/app.yaml -source=config/source.yaml

docker-run-detached:
	docker run -d \
	--ulimit nofile=65536:65536 \
	-p 8181:8181 \
	-p 8282:8282 \
	-v $(PWD)/config/source.yaml:/app/config/source.yaml:ro \
	-v $(PWD)/config/app.yaml:/app/config/app.yaml:ro \
	-v $(PWD)/data:/app/data \
	-v $(PWD)/logs:/app/logs \
	--name bot-go \
	$(DOCKER_IMAGE):$(VERSION) \
	./bot-go -app=config/app.yaml -source=config/source.yaml

docker-run-with-workdir:
	@if [ -z "$(WORKDIR)" ]; then echo "Usage: make docker-run-with-workdir WORKDIR=/path/to/workdir"; exit 1; fi
	docker run -it --rm \
	--ulimit nofile=65536:65536 \
	-p 8181:8181 \
	-p 8282:8282 \
	-v $(PWD)/config/source.yaml:/app/config/source.yaml:ro \
	-v $(PWD)/config/app.yaml:/app/config/app.yaml:ro \
	-v $(WORKDIR):/app/workdir \
	-v $(PWD)/data:/app/data \
	-v $(PWD)/logs:/app/logs \
	--name bot-go \
	$(DOCKER_IMAGE):$(VERSION) \
	./bot-go -app=config/app.yaml -source=config/source.yaml -workdir=/app/workdir

docker-stop:
	docker stop bot-go || true
	docker rm bot-go || true

docker-logs:
	docker logs -f bot-go

# Docker Compose targets for full stack
docker-compose-up:
	docker-compose up --build -d

docker-compose-down:
	docker-compose down

docker-compose-logs:
	docker-compose logs -f

# Docker distribution targets
docker-tag:
ifdef REGISTRY
	docker tag $(DOCKER_IMAGE):$(VERSION) $(REGISTRY)/$(DOCKER_IMAGE):$(VERSION)
	docker tag $(DOCKER_IMAGE):$(VERSION) $(REGISTRY)/$(DOCKER_IMAGE):latest
else
	@echo "REGISTRY not set. Use: make docker-tag REGISTRY=your-registry.com"
endif

docker-push: docker-tag
ifdef REGISTRY
	docker push $(REGISTRY)/$(DOCKER_IMAGE):$(VERSION)
	docker push $(REGISTRY)/$(DOCKER_IMAGE):latest
else
	@echo "REGISTRY not set. Use: make docker-push REGISTRY=your-registry.com"
endif

# Build and push in one command
docker-release: docker-build docker-push
	@echo "Released $(DOCKER_IMAGE):$(VERSION) to $(REGISTRY)"

setup-python-env:
	python3 -m venv $(VENV_DIR)
	@echo "Python virtual environment created at $(VENV_DIR)"
	@echo "To activate: source $(VENV_DIR)/bin/activate"

install-lsp-servers: setup-python-env
	@echo "Installing Go language server (gopls)..."
	go install golang.org/x/tools/gopls@latest
	@echo "Installing Python language server in virtual environment..."
	$(VENV_DIR)/bin/pip install python-lsp-server
	@echo "Installing TypeScript language server..."
	npm install -g typescript-language-server typescript
	@echo "All language servers installed successfully!"
	@echo "Remember to activate Python venv when needed: source $(VENV_DIR)/bin/activate"
