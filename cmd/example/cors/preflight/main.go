package main

import (
	"flag"
	"log"
	"net/http"
)

const html = `
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
</head>
<body>
	<h1>Preflight CORS</h1>
	<div id="output"></div>
	<script>
		document.addEventListener('DOMContentLoaded', function() {
			fetch("http://localhost:4000/v1/tokens/authentication", {
				method: "POST",
				<!--包含一个Content-Type: application/json 请求头，这在一个简单的跨域请求中是不被允许的，会触发一个对API的预飞行请求-->
				headers: {
					'Content-Type': 'application/json'
				},
				body: JSON.stringify({
					email: 'alice@example.com',
					password: 'LZ251210*'
				})
			}).then(
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
	addr := flag.String("addr", ":9000", "Server address")
	flag.Parse()
	log.Printf("starting server on %s", *addr)

	err := http.ListenAndServe(*addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(html))
	}))
	log.Fatal(err)
}
