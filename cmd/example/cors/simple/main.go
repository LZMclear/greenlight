package main

import (
	"flag"
	"log"
	"net/http"
)

// 定义一个包含网页HTML字符的常量
// JavaScript从/v1/healthcheck获取json数据
const html = `
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
</head>
<body>
	<h1>Simple CORS</h1>
	<div id="output"></div>
	
	<script>
		document.addEventListener('DOMContentLoaded', function() {
			<!--使用fetch函数像healthcheck发出请求，fetch方法异步工作返回一个promise，随后在promise上使用then方法设置两个回调函数-->
			fetch("http://localhost:4000/v1/healthcheck").then(
				function (response) {
					response.text().then(function (text) {
						document.getElementById("output").innerHTML = text;
					});
				},
				function(err) {
					document.getElementById("output").innerHTML = err;
				}
			);
		});
	</script>
</body>
</html>`

func main() {
	addr := flag.String("addr", ":9001", "server address")
	flag.Parse()
	log.Printf("starting server on %s", *addr)
	err := http.ListenAndServe(*addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(html))
	}))
	log.Fatal(err)

}
