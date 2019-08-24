/*
This is the test package for CertificateService. You can run it by issuing the command
go test -v CertificateService

Before running the test, you may need to run
go get github.com/Pallinder/go-randomdata

This test file essentially runs an emulation  for a Certificate server that serves and maintains security certificates for domain servers

*/

package CertificateService

import (
	"fmt"
	//go get github.com/Pallinder/go-randomdata
	"github.com/Pallinder/go-randomdata"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sync"
	"testing"
	"time"
)

/*
 Along with the imported Pallinder/go-randomdata library, this helper function
creates a random extension for domain names  {domain name}.[ext}
*/
func randExt() string {
	ext := make([]string, 0)
	ext = append(ext,
		".com",
		".net",
		".au",
		".us",
		".eu",
		".fanatics")
	rand.Seed(time.Now().UnixNano()) // initialize global pseudo random generator
	return ext[rand.Intn(len(ext))]
}

/*
Testserver tests both the http server and the redis database for a valid connection that the
rest of the test functions can use.

Testserver will start the http server manually and test the connection, However:

Redis must be started before hand. If Redis is not installed, the easiest way to handle redis is to use docker.

If you have Docker installed, you can start redis from the command line and have the data persist between sessions.

docker run --name some-redis -d -p 6379:6379 redis redis-server --appendonly yes

*/
func TestServer(t *testing.T) {

	db := NewDbConn()

	go db.OpenHTTPServer()
	resp, err := http.Get("http://localhost:8080")
	if err != nil {
		t.Fatalf("There was a problem opening http server, Please check your configuration and re run the test \n" + err.Error())
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	t.Logf(string(body[:]))

	redisOK := PingRedis(&db)
	if !redisOK {
		t.Fatalf("Redis could not be pinged. Please start Redis befor running these tests")
	}

	testCreateCerts(&db)
}

/*
TestCreateCerts tests the creation of certs, specifically,
the SIMULTANEOUS creation of domains, despite the built in 10
second delay after the creation of a cert.

Simultanaity is achieved by using waitgroups and goroutines.

Just imagine if 200 requests were made within in few seconds during the world series. The first user would have to
wait 33 minutes before they could validate their connection. This way, each and every simultaneous user would
wait only 10 seconds ( or slightly more.

*/
func testCreateCerts(db *dbConn) {
	//create 100 domain name

	fmt.Println("Simultaneous creation of 10 domains. Should take 10 seconds due to the delay requirements in the specification.")
	var wg = sync.WaitGroup{}
	for i := 0; i < 9; i++ {
		//need a seperate wait group for each iteration
		wg.Add(1)
		//simultaneous creation of domains
		go func() {
			defer wg.Done()
			resp, _ := http.Get("http://localhost:8080/certcreate/" + randomdata.SillyName() + randExt())
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)
			str2 := string(body[:])
			fmt.Println(str2)
		}()
	}
	//wait for all go routines to return, otherwise, the system will panic
	wg.Wait()

	testDomains(db)
}

// TestDomain; the fmt.PrintLn messages below provide good documentation for this function
func testDomains(db *dbConn) {
	fmt.Println("Testing each cert that we created through an http connection.")
	fmt.Println("localhost:80808/cert/{Domain}. CERTSERVER.FAN will be created seperatel by certificate service for its own use.")

	//retrieve all the domains in the redis cache
	x := db.GetAll()
	for i, v := range x {
		/* Each value of x contains a 1value for the domain name and 1 for the expiration date
		   I'm only seeking to return the Domain names. The Domain names are all the even valued
		   number, hence the i mod 2 expression here.
		*/
		if i%2 == 0 {
			resp, _ := http.Get("http://localhost:8080/cert/" + v)
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)
			str2 := string(body[:])
			fmt.Println(str2)
		}
	}
	fmt.Print("\n\n")
	fmt.Println("Each certificate expires after 10 minutes.")
	fmt.Println("This simulation lasts just over 11 minutes")
	fmt.Println("We are waiting to see if:")
	fmt.Println("-the server renews its cert automatically per the requirements")
	fmt.Println("-the regular certs expire on their own")
	fmt.Println("You can cancel the program now if you are satisfied, or, ")
	fmt.Println("during this simulation, try opening a browser ")
	fmt.Println("and creating a cert by going to localhost:8080/certcreate/{domain}.")
	fmt.Println("and finally testing it by going to localhost:8080/cert/{domain}")
	fmt.Println("Valid domains include any alphanumeric combination of 1-62 character, followed ")
	fmt.Println("by a '.' and finally by another alphanumeric combination of 2-62 characters.")
	fmt.Println("Examples: ")
	fmt.Println("Valid: Fanatics.com")
	fmt.Println("Invalid:  Fanatics (no extension)")
	fmt.Println("Invalid Fanatics.co.uk (too many extensions).")
	fmt.Println("WAITING FOR 11 MINUTES.......")
	time.Sleep(time.Minute * 11)

	testFinal(db)

}

func testFinal(db *dbConn) {
	fmt.Print("\n\n")
	fmt.Println("Testing if the domains expired, and the server certificate renewed")
	//retrieve all the domains in the redis cache
	x := db.GetAll()
	for i, v := range x {
		/* Each value of x contains a 1value for the domain name and 1 for the expiration date
		   I'm only seeking to return the Domain names. The Domain names are all the even valued
		   number, hence the i mod 2 expression here.
		*/
		if i%2 == 0 {
			resp, _ := http.Get("http://localhost:8080/cert/" + v)
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)
			str2 := string(body[:])
			fmt.Println(str2)
		}
	}

	fmt.Println("CERTSERVER.FAN should be the only certificate that hasn't expired.")
}
