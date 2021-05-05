
# https\_capture: A HTTP(S) capturing MITM proxy that write contents to files

`https_capture` is a tiny HTTP/HTTPS MITM proxy utility to log, capture and save HTTP(s) communications to files.


## Install and setup

### install the command
Be sure to have the latest version of [go-language](https://golang.org/) command line.
Then do the following.
```
go install github.com/mixcode/https_capture@latest
```

Or, you may pull the source and do `go generate && go build`.


### create a dummy Root CA certificate to peek HTTPS communications

To peek HTTPS communications, you have to tell your web client that https\_capture proxy is trustable. You can do it by creating and installing a self-signed (insecure) Root CA certificate of https\_proxy to the web client.

This command creates a new cert and saves it to 'my\_insecure\_root\_ca.cer'
```
https_capture -generate-cert my_insecure_root_ca.cer
```

You have to install the generated cert (in this case `my_insecure_root_ca.cer`) to your web client or OS. Refer to the client or OS manuals for details.


### start the proxy server

A quick example:
```
$ https_capture -addr=:38080 -dir=./captured -log=log.txt -c -tee my_insecure_root_ca.cer
```

Run the proxy with the generated cert on the machine's `:38080` port. The HTTP requests will be stored in the `./captured` directory. The `-tee` makes the log echoed to STDOUT. The `-c` will clear the capturing directory on start.

To see available options, do `https_capture -help`.


### start sending data through the proxy

Modify the HTTP proxy setting of your web client (or OS) to the proxy program just started (in the example above, `localhost:38080`). If everything is OK, then the log file and captured files will appear on the capturing directory once you visit some web pages.


## Connection log

For each HTTP(s) connection, the request headers and response headers are logged to a log file. 
Logs are consist of tab-indented lines.
Lines with no tabs, and an RFC3339 timestamp, and a sequence number for each connection (in a square bracket) means the start or the end of an HTTP connection.
The contents of a connection are written when the connection ends. HTTP headers appear as tab-indented lines.
If there are any HTTP bodies, then the saved filename is shown at the end of the chunk.


This is an example of captured logs.
```
2021-04-28T14:55:11+09:00 [4] start GET https://www.google.com/
2021-04-28T14:55:12+09:00 [4] end GET https://www.google.com/
	==== Req header ====
		Accept-Language: [en-US,en;q=0.9]
		Upgrade-Insecure-Requests: [1]
		User-Agent: [Mozilla/5.0 (..........)]
		Accept: [text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8]
		X-Client-Data: [(.........)]
		Accept-Encoding: [gzip, deflate, br]
		Connection: [keep-alive]
		Cache-Control: [max-age=0]
		Cookie: [1P_JAR=2021-04-28-05; UULE=.............]
	==== Resp header ====
		Cache-Control: [private]
		X-Xss-Protection: [0]
		X-Frame-Options: [SAMEORIGIN]
		Expires: [Wed, 28 Apr 2021 05:55:12 GMT]
		Set-Cookie: [1P_JAR=2021-04-28-05; expires=Fri, 28-May-2021 05:55:12 GMT; path=/; domain=.google.com; Secure; SameSite=none]
		Alt-Svc: [h3-29=":443"; ma=2592000,h3-T051=":443"; .....]
		Content-Type: [text/html; charset=UTF-8]
		Strict-Transport-Security: [max-age=31536000]
		Transfer-Encoding: [chunked]
		Server: [gws]
		Connection: [close]
		Date: [Wed, 28 Apr 2021 05:55:12 GMT]
	---- Resp body ----
		(saved to: [000004_b_unknown.bin])
```


## Recorded HTTP bodies

Each HTTP request body and response body are stored as a file.
The filename of saved body contents begins with the sequence number of the connection.
Following the sequence number, there is a mark, "\_a\_" or "\_b\_", which means request body and response body.

This is an example of stored files.
```
$ ls ./captured
-rw-r--r--. 1 mixcode     44 04-28 19:39 000245_b_gn.gif
-rw-r--r--. 1 mixcode     35 04-28 19:39 000246_b_unknown.bin
-rw-r--r--. 1 mixcode     43 04-28 19:39 000247_b_vad.gif
-rw-r--r--. 1 mixcode   1477 04-28 19:39 000248_a_request.bin
-rw-r--r--. 1 mixcode    239 04-28 19:39 000248_b_yql.txt
-rw-r--r--. 1 mixcode  35362 04-28 19:39 000252_a_request.bin
-rw-r--r--. 1 mixcode    239 04-28 19:39 000252_b_yql.txt
-rw-r--r--. 1 mixcode     44 04-28 19:39 000253_b_gn.gif
-rw-r--r--. 1 mixcode    727 04-28 19:39 000255_b_offer.js
-rw-r--r--. 1 mixcode     43 04-28 19:39 000257_b_p.gif
```


## The Internal

This program is rather a placeholder for a customizable HTTP debugger than a standalone utility. The core proxy function of this utility is based on [elazarl's goproxy](https://github.com/elazarl/goproxy) library, and this utility wraps the functions into a command-line program.

All HTTP data goes through the callback functions in `handler.go`. Currently, all data are blindly saved into the capture directory. However, you may find it is trivial to filter and select the data of your interest.


