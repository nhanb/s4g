build:
	go build -o dist/

watch:
	find . -name '*.go' -or -name '*.js' -or -name '*.tmpl' -or -name '*.dj' \
	| entr -rc go run .

# Cheating a little because the djot.js repo on github does not provide builds
update-djot:
	curl -L 'https://djot.net/playground/djot.js' > djot/js/djot.js

clean:
	rm -rf dist/*
