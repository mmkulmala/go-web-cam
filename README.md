# go-web-cam
Simple golang based webcam app. First designed as a prototype for videoconferencing.

## Structure
Client folder has client side code (client.go and static html page for showing web cam in browser) and server folder the web cam app server code (server.go and static html page to show web cam for client calling). All is Golang (see go.sum or specific version of libs used).

## Start client and server
Simple run the following shell script:
```bash
./start.sh
```
This script starts both server and client and is basic web cam app to use in video communication.

Or one can run them separately if one likes: see below.

Client starting:
```bash
./client-start.sh
```

Server starting:
```bash
./server-start.sh
```
