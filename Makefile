APP_NAME=processjobqueue

build:
	go build -o $(APP_NAME) main.go

run:
	go run main.go

docker-build:
	docker build -t $(APP_NAME):latest .

docker-run:
	docker run -p 8080:8080 -v $(PWD)/jobs:/app/jobs $(APP_NAME):latest
