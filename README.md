
# http_capture: What is this?

A tiny HTTP/HTTPS MITM proxy utility to log, capture and save HTTP communications to files.


# How to use

```
./https_capture -addr=:38080 -dir=./captured -log=log.txt -t
```

Run the program, then set your web proxy to the machine's `:38080` port. Then the HTTP requests will be stored in the `./captured` directory. The `-t` makes the log echoed to STDOUT.



# Connection log

For each HTTP(s) connection, the request headers and response headers are logged to a log file. 

Logs are chunks of tab-indented lines.
Lines with no tabs, and an RFC3339 timestamp, and a sequence number for each connection (in a square bracket) are the beginning of a new chunk.

Each chunk is the start of an HTTP connection or the end of an HTTP connection. The contents of a connection are stored when the connection ends. HTTP headers are written along with the end of the connection as tab-indented lines.
Each HTTP request body and response body are stored as a file. The filename of saved body contents begins with the sequence number of the connection. Following the sequence number, there is a mark, "\_a\_" or "\_b\_", each means request body and response body.


This is an example of captured logs.
```
2021-04-28T14:55:11+09:00 [4] start GET https://www.google.com/
2021-04-28T14:55:12+09:00 [4] end GET https://www.google.com/
	==== Req header ====
		[Accept-Language]: [en-US,en;q=0.9]
		[Upgrade-Insecure-Requests]: [1]
		[User-Agent]: [Mozilla/5.0 (..........)]
		[Accept]: [text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8]
		[X-Client-Data]: [(.........)]
		[Accept-Encoding]: [gzip, deflate, br]
		[Connection]: [keep-alive]
		[Cache-Control]: [max-age=0]
		[Cookie]: [1P_JAR=2021-04-28-05; UULE=.............]
	==== Resp header ====
		[Cache-Control]: [private]
		[X-Xss-Protection]: [0]
		[X-Frame-Options]: [SAMEORIGIN]
		[Expires]: [Wed, 28 Apr 2021 05:55:12 GMT]
		[Set-Cookie]: [1P_JAR=2021-04-28-05; expires=Fri, 28-May-2021 05:55:12 GMT; path=/; domain=.google.com; Secure; SameSite=none]
		[Alt-Svc]: [h3-29=":443"; ma=2592000,h3-T051=":443"; .....]
		[Content-Type]: [text/html; charset=UTF-8]
		[Strict-Transport-Security]: [max-age=31536000]
		[Transfer-Encoding]: [chunked]
		[Server]: [gws]
		[Connection]: [close]
		[Date]: [Wed, 28 Apr 2021 05:55:12 GMT]
	---- Resp body ----
		(saved to: [000004_b_unknown.bin])
```


# Capturing HTTPS communications

__!! WARNING !! DO THIS ONLY IF YOU REALLY KNOW WHAT YOU ARE DOING !!__

As of the nature of HTTPS, you cannot interfere with HTTPS communications normally.
If you really want to capture HTTPS connections, you have to install the [goproxy](https://github.com/mixcode/goproxy)'s [root cert](https://github.com/mixcode/goproxy/blob/master/ca.pem) to your device as a Trusted Root certificate to do a MITM-attack on HTTPS connections.
(If you does not know how to do it, then Google it)

