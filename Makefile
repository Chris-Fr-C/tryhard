

default:
	go build
win:
	GOOS=windows GOARCH=386 go build -o tryhard-windows.exe
