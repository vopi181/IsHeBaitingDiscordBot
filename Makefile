ifeq ($(OS),Windows_NT) 
	detected_OS := Windows
else
	detected_OS := $(shell sh -c 'uname 2>/dev/null || echo Unknown')
endif



folderstruct:
	mkdir dist
IsHeBaitingDiscordBot: folderstruct
	go build
	cp IsHeBaitingDiscordBot* dist/
boilerbins:	folderstruct
	mkdir dist/boilerbins_linux
	cp boilerbins_linux/* dist/boilerbins_linux/
	mkdir dist/boilerbins_windows
	cp boilerbins_windows/* dist/boilerbins_windows
	mkdir dist/output

dist:	IsHeBaitingDiscordBot boilerbins
	# ifeq ($(detected_OS),Windows)
		pwsh.exe -c "Compress-Archive dist -DestinationPath ./dist.zip"
	# endif
	# ifeq ($(detected_OS),Linux)
	# 	zip -r dist.zip dist
	# endif

default: dist




clean:
	rm dist/boilerbins_linux/*
	rm dist/boilerbins_windows/*
	rm dist/boilerbins_linux/
	rm dist/boilerbins_windows/
	rm dist/IsHeBaitingDiscordBot*
	rm dist/
	rm dist.zip