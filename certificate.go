/*
Package CertificateService provides functions to:

Start an http server on the localhost port 8080.

Create and maintain a pooled connection to a redis server.
Ping the redis server to see if its alive.

Create domain certificates with a 10 minute expiration date.
Retrieve a domain for validation purposes,
Provide an http handler to receive and process these 'Create' and 'Retrieve' requests

*/

package CertificateService

import (
	"encoding/binary"
	"fmt"
	"regexp"

	//imported pagckage, run go get github.com/gomodule/redigo/redis
	"github.com/gomodule/redigo/redis"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

/*
The 'pool' and 'newpool' function are used to maintain a system of connections to a redis server.

'pool' and 'newpool' uses the imported redigo package to talk to the redis database. Make sure
this package is imported before using.

Redis must be started before using any functions in this package. If you have docker, redis is simple
to use:

docker run --name some-redis -d -p 6379:6379 redis redis-server --appendonly yes

This docker command will ensure that redis start on port 6379 (-p 6379:6379) and will persisit data between sessions.

For future versions, a better strategy for maintaining database connections should be considered
https://www.alexedwards.net/blog/organising-database-access

*/

var pool = newPool()

/*
newPool closely follows the recommended initiation of the  connection pool in the redis.Pool documentation.
For more information, view the redigo redis.Pool documentation on the imported package
*/
func newPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:   80,
		MaxActive: 12000, // max number of connections
		Dial: func() (redis.Conn, error) {
			// by default, redis starts on port 6379. If you have it started on a diff
			c, err := redis.Dial("tcp", "192.168.99.100:6379")
			if err != nil {
				fmt.Println(err.Error())
			}
			return c, err
		},
	}
}

// certserver creates a root certificate for the certificate service itself

var certServer = wrapCertServer()

/*
	This function just wraps a call to newCertServer.
 	Needed handle the time.Afterfunc line which requires a func() with no return value
*/
func wrapCertServer() bool {
	newCertServer()
	return true
}

//Make sure the http servers certificate has been created and is up to date
func newCertServer() {
	//this next line creates OR renews a certificate
	_, err := createCert("CERTSERVER.FAN")
	if err != nil {
		log.Fatal(err)
	}
	/*
		Each certificate is created with a 10 minute expiration date. Make sure
		the server is renewed ever 9 minutes
	*/
	time.AfterFunc(time.Minute*9, newCertServer)

	return
}

/*
OpenHTTPServer provides:

An http server.
An http handler for routing http requests.
*/
func OpenHTTPServer() {

	http.HandleFunc("/", httpHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))

}

/*
createCert serves two purposes:
1: to create a cert if it doesn't exist
2: renew a cert if it exists, but has expired
*/
func createCert(domainName string) (string, error) {
	/*
		Use a pooled connection to redis and close the
		connection when the function exits.
	*/
	conn := pool.Get()
	defer conn.Close()

	// set or renew the expiration date/time for the cert
	expires := time.Now().Add(time.Minute * 10)

	/*
		connect and store the cert and the expiration date
		the expiration date time string are rather large. We're encoding it here as byte slice
		to help protect against parsing errors or modifying the time in unwanted ways.
	*/
	resp, err := redis.String(conn.Do("HMSET", "Domain", domainName, encode(expires)))
	if err != nil {
		log.Fatal(err)
	}

	return resp, err

}

/*
getCert queries the redis cache for a domain name and expiration date. The user
will send a domain name and retrieve an expiration time if the domain exists, otherwise,
and error is thrown (usually something like "REDIGO: NIL RETURNED") or a connection error.

A good use for this is, say a client web browser trying to validate a domain certificate
to establish a trusted connection.
*/

func getCert(domainName string) (time.Time, error) {

	/*
		Use a pooled connection to redis and close the
		connection when the function exits.
	*/
	conn := pool.Get()
	defer conn.Close()

	//retrieve the expiration and any errors
	expires, err := redis.Bytes(conn.Do("HGET", "Domain", domainName))
	if err != nil {
		return time.Now(), err
	}

	/*
			Return the expiration data and any errors.
		    decode translates the expiration, stores as a Byte slice, to a string
	*/
	return decode(expires), err
}

/*
'httpHandler takes routes a request through a tree of possible options
should be able to handle all scenarios and edge cases.........
*/

func httpHandler(w http.ResponseWriter, r *http.Request) {

	// force the request URI to uppercase for easy comparison tests
	temp := strings.ToUpper(r.RequestURI)

	// final step after results of the decision tree below
	finalStep := func(full string, prefix string, getorset string) {
		//trim the /CERT/ OR /CERTCREATE/ prefix from the decision tree below
		DomainName := strings.TrimPrefix(full, prefix)
		// writes the final response string after a request to create or retrieve a domain
		io.WriteString(w, "<h1>"+redisResponse(DomainName, getorset)+"</h1>")
	}

	//decision tree routing
	if strings.Contains(temp, "/CERTCREATE/") {
		finalStep(temp, "/CERTCREATE/", "CREATE")
	} else if strings.Contains(temp, "/CERT/") {
		finalStep(temp, "/CERT/", "RETRIEVE")
	} else {
		io.WriteString(w, "<h1> server is live, Send a valid certification request  to localhost:8080/cert/{domain} or localhost:8080/certcreate/{domain} </h1>")
	}
}

/*
Similar to and working in conjunction with the decision tree from httpHandler above.
this function sends and receives responses from the redis cache.
*/
func redisResponse(domainName string, createOrRetrieve string) string {
	/*
		Valid domains include any alphanumeric combination of 1-62 character, followed
		by a '.' and finally by another alphanumeric combination of 2-62 characters.
		Examples:
		Valid: Fanatics.com
		Invalid:  Fanatics (no extension)
		Invalid Fanatics.co.uk (too many extensions).
	*/
	validate, _ := regexp.Compile("^[a-zA-Z0-9|-]{0,61}[a-zA-Z0-9]\\.[a-zA-Z]{2,62}$")
	if !validate.MatchString(domainName) {
		return ("Invalid domain name: " + domainName)
	}

	if createOrRetrieve == "RETRIEVE" {
		return retrieve(domainName)
	} else { // CREATE is selected, create the domain
		return create(domainName)
	}

}

/*
'retrieve' is part of the redisResponse decision tree above
*/
func retrieve(domainName string) string {
	//attempt to retrieve the domainName query from the redis cache
	expire, err := getCert(domainName)
	if err != nil {
		//domain doesn't exist in redis cach
		if strings.ToUpper(err.Error()) == "REDIGO: NIL RETURNED" {
			return "This domain doesn't exist: " + domainName + ". Submit a cert request to localhost:8080/certcreate/{domain}"
		} else {
			return err.Error()
		}
	} else if expire.Before(time.Now()) {
		//domain exists but has expired
		return "foo{" + domainName + "}" + " expired, not trusted"
	} else {
		return "foo{" + domainName + "}"
	}
}

/*
'retrieve' is part of the redisResponse decision tree above
*/
func create(domainName string) string {
	// issue a create request to the redis cache
	resp, err := createCert(domainName)
	// required delay set out by the specification
	time.Sleep(time.Second * 10)
	if err != nil {
		return err.Error()
	} else {
		return resp
	}
}

/*
 Public access method to see if Redis is alive
*/
func PingRedis() bool {
	/*
		Use a pooled connection to redis and close the
		connection when the function exits.
	*/
	conn := pool.Get()
	defer conn.Close()

	/*
		Reply would be "PONG", but an error will be thrown if "PONG" isn't recived
	*/
	_, err := conn.Do("PING")
	if err != nil {
		return false
	} else {
		return true
	}
}

//helper functions
// encode marshals a time.

func encode(t time.Time) []byte {
	buf := make([]byte, 8)
	u := uint64(t.Unix())
	binary.BigEndian.PutUint64(buf, u)
	return buf
}

// decode unmarshals a time.
func decode(b []byte) time.Time {
	i := int64(binary.BigEndian.Uint64(b))
	return time.Unix(i, 0)
}

/*
GetAll retrieves all of the domains stored in the redis database. This is just provided for
convenience of testing.
*/
func GetAll() []string {

	conn := pool.Get()
	defer conn.Close()

	data, err := redis.ByteSlices(conn.Do("HGETALL", "Domain"))

	if err != nil && err.Error() != "redigo: nil returned" {
		log.Fatalf("error: %v", err)
	}
	var c = make([]string, len(data))
	for i, v := range data {
		if i%2 == 0 {
			c[i] = string(v)
		} else {
			c[i] = "\n"
		}
	}

	return c
}
