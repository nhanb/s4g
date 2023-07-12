build:
	go build -o dist/

watch:
	find . -name '*.go' -or -name '*.js' -or -name 'livereload.html' \
	| entr -rc go run .

watch-theme:
	find theme/* | entr -c rsync -av theme/ www/_theme/

# Cheating a little because the djot.js repo on github does not provide builds
update-djot:
	curl -L 'https://djot.net/playground/djot.js' > djot/js/djot.js

clean:
	rm -rf dist/*
