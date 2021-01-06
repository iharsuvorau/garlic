WIN64_DIR := build/windows_amd64
OSX_DIR := build/darwing_amd64
BIN_NAME := garlic

windows:
	rm -r $(WIN64_DIR)/*; GOOS=windows GOARCH=amd64 go build -o $(WIN64_DIR)/$(BIN_NAME).exe .; cp -r data $(WIN64_DIR)

darwin:
	rm -r $(OSX_DIR)/*; GOOS=darwin GOARCH=amd64 go build -o $(OSX_DIR)/$(BIN_NAME) .; cp -r data $(OSX_DIR)

run:
	go run .