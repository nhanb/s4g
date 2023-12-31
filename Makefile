build:
	go build -o dist/

watch:
	fd -E docs -E theme | entr -rc -s \
		'go build -o dist/ && ./dist/s4g serve -f docs -p 3338'

watch-theme:
	find theme/* | entr -c rsync -av theme/ docs/_s4g/theme/

# Cheating a little because the djot.js repo on github does not provide builds
update-djot:
	curl -L 'https://djot.net/playground/djot.js' > djot/js/djot.js

clean:
	rm -rf dist/*
