chrome-extension.zip:
	zip -r chrome-extension.zip extensions/chrome

clean:
	rm chrome-extension.zip

favicon:
	wget https://raw.githubusercontent.com/emarref/webicon/master/webicon.sh -O webicon.sh
	chmod +x webicon.sh
	cd assets/favicons && ../../webicon.sh ../../favicon.png
	rm webicon.sh

dev:
	go test ./... && go build && ./ancientcitadel
