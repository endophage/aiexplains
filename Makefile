.PHONY: build frontend backend run dev clean

# Build everything
build: frontend backend

# Build the React frontend
frontend:
	cd frontend && npm run build

# Build the Go backend binary
backend:
	cd backend && go build -o ../aiexplains .

# Build and run the server (serves frontend from ./frontend/dist)
# Uses --localexec by default; set ANTHROPIC_API_KEY and pass LOCALEXEC= to use the SDK instead
LOCALEXEC ?= --localexec

run: build
	./aiexplains serve $(LOCALEXEC)

# Dev mode: run both servers with hot reload
# Terminal 1: make dev-backend
# Terminal 2: make dev-frontend
dev-backend:
	cd backend && go run . serve $(LOCALEXEC)

dev-frontend:
	cd frontend && npm run dev

clean:
	rm -f aiexplains
	rm -rf frontend/dist
