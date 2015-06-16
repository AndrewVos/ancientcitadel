all: extensions/chrome.zip build

favicon:
	wget https://raw.githubusercontent.com/emarref/webicon/master/webicon.sh -O webicon.sh
	chmod +x webicon.sh
	cd assets/favicons && ../../webicon.sh ../../favicon.png
	rm webicon.sh

go_dependencies = $(shell find . -type f -not -name 'ancientcitadel' -not -path "./extensions/*" -not -path "./.git/*")
ancientcitadel: ${go_dependencies}
	go test ./...
	go build

build: ancientcitadel

dev: build
	./ancientcitadel

includes = $(wildcard extensions/chrome/*)
extensions/chrome.zip: ${includes}
	rm extensions/chrome.zip || :
	zip -r extensions/chrome.zip extensions/chrome
