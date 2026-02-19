.PHONY: all frontend backend clean dev-frontend dev-backend

GO := PATH="/home/chris/go-sdk/go/bin:$(PATH)" go

all: c3

frontend:
	cd frontend && npm ci && npm run build

c3: frontend
	$(GO) build -o c3 .

backend:
	$(GO) build -o c3 .

clean:
	rm -f c3
	rm -rf frontend/dist frontend/node_modules

dev-frontend:
	cd frontend && npm run dev

dev-backend:
	$(GO) run . --tmux-target=$(TMUX_TARGET) --listen-addr=:8080
