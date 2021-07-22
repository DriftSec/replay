## replay burp requests from a file
```
Usage of replay:
  -R value
        Replace sting in request file. Use multiple times (-R infile=./test.txt will replace {{infile}} with ./test.txt)
  -https
        HTTPS request, defaults to HTTP
  -r string
        The request file to replay
```


# string replacing
### add tags to the request {{tag_name}}
```
POST /hackem.php HTTP/1.1
Host: 127.0.0.1:8000
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:79.0) Gecko/20100101 Firefox/79.0
Accept: text/javascript, application/javascript, application/ecmascript, application/x-ecmascript, */*; q=0.01
Accept-Language: tr-TR,tr;q=0.8,en-US;q=0.5,en;q=0.3
Accept-Encoding: gzip, deflate
X-Requested-With: XMLHttpRequest
Connection: close
Cookie: secret={{fakecookie}}

data={{some_data}}
```

### use -R tagname=value
```
go run *.go -r test.req -R fakecookie=blahblahcookieDATA -R some_data=this_is_some_randome_data
```
