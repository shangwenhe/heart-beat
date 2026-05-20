APP      := schedule-app
DB       := schedule.db
PORT     := 8080

.PIANIFY: build run clean dev reset-db

build:
	go build -o $(APP) .

run: build
	./$(APP)

dev:
	go run .

clean:
	rm -f $(APP) $(DB) server.log

reset-db:
	rm -f $(DB)
	@echo "数据库已重置，下次启动时自动初始化"
