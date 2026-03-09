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
run: build
	./aiexplains serve

# Dev mode: run both servers with hot reload
# Terminal 1: make dev-backend
# Terminal 2: make dev-frontend
dev-backend:
	cd backend && go run . serve

dev-frontend:
	cd frontend && npm run dev

clean:
	rm -f aiexplains
	rm -rf frontend/dist
