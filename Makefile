.PHONY: build frontend backend run run-webview app icns dev-backend dev-frontend clean

APP_NAME   = AIExplains
APP_BUNDLE = $(APP_NAME).app
ICON_SRC   = assets/icon.png
ICNS_OUT   = assets/AppIcon.icns

# Build everything (no embed)
build: frontend backend

# Build the React frontend
frontend:
	cd frontend && npm run build

# Build the Go backend binary (filesystem frontend, for running directly)
backend:
	cd backend && go build -o ../aiexplains .

run: build
	./aiexplains serve

run-webview: build
	./aiexplains serve --webview

# Build a self-contained .app bundle with embedded frontend and webview
app: frontend icns
	# Embed frontend into binary
	cp -r frontend/dist backend/cmd/frontend
	cd backend && go build -tags embed -o ../$(APP_NAME)-bin .
	rm -rf backend/cmd/frontend
	# Assemble bundle
	rm -rf $(APP_BUNDLE)
	mkdir -p $(APP_BUNDLE)/Contents/MacOS
	mkdir -p $(APP_BUNDLE)/Contents/Resources
	mv $(APP_NAME)-bin $(APP_BUNDLE)/Contents/MacOS/$(APP_NAME)
	cp $(ICNS_OUT) $(APP_BUNDLE)/Contents/Resources/AppIcon.icns
	cp assets/Info.plist $(APP_BUNDLE)/Contents/Info.plist
	# Ad-hoc code sign so Gatekeeper doesn't block it
	codesign --sign - --force --deep $(APP_BUNDLE)
	@echo "Built $(APP_BUNDLE)"

# Convert assets/icon.png → assets/AppIcon.icns
icns: $(ICNS_OUT)
$(ICNS_OUT): $(ICON_SRC)
	$(eval ICONSET := $(shell mktemp -d).iconset)
	mkdir -p $(ICONSET)
	sips -z 16   16   $(ICON_SRC) --out $(ICONSET)/icon_16x16.png       >/dev/null
	sips -z 32   32   $(ICON_SRC) --out $(ICONSET)/icon_16x16@2x.png    >/dev/null
	sips -z 32   32   $(ICON_SRC) --out $(ICONSET)/icon_32x32.png       >/dev/null
	sips -z 64   64   $(ICON_SRC) --out $(ICONSET)/icon_32x32@2x.png    >/dev/null
	sips -z 128  128  $(ICON_SRC) --out $(ICONSET)/icon_128x128.png     >/dev/null
	sips -z 256  256  $(ICON_SRC) --out $(ICONSET)/icon_128x128@2x.png  >/dev/null
	sips -z 256  256  $(ICON_SRC) --out $(ICONSET)/icon_256x256.png     >/dev/null
	sips -z 512  512  $(ICON_SRC) --out $(ICONSET)/icon_256x256@2x.png  >/dev/null
	sips -z 512  512  $(ICON_SRC) --out $(ICONSET)/icon_512x512.png     >/dev/null
	cp $(ICON_SRC) $(ICONSET)/icon_512x512@2x.png
	iconutil -c icns $(ICONSET) -o $(ICNS_OUT)
	rm -rf $(ICONSET)

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
	rm -rf $(APP_BUNDLE)
	rm -f $(ICNS_OUT)
