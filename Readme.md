# Certificate Service

This package was created in response to a programming challenge.

Package CertificateService provides functions to:

--Start an http server on the localhost port 8080.
--Create and maintain a pooled connection to a redis server.
--Ping the redis server to see if its alive.
--Create domain certificates with a 10 minute expiration date.
--Retrieve a domain for validation purposes,
--Provide an http handler to receive and process these 'Create' and 'Retrieve' requests

The code is only stub code for now. The 'certificate' is only a key value pair stored in a 
redis cache containing the domain name and expiration date. 


## Testing the package

The certificate_test.go runs a simulation of a generic certificate server. To run it:

1. Download the source code to your golang path src directory.

    `/src/Certificateservice`/

2. Import the redigo package.

   `go get github.com/gomodule/redigo/redis`

3. Import a randomizer.

    `go get github.com/Pallinder/go-randomdata`

4. Start redis. If you have docker installed, this is easy
    
   ` docker run --name some-redis -d -p 6379:6379 redis redis-server --appendonly yes`

5. Finally, test the package, The emulation lasts a little over 11 minutes to.
Read the instructions as the test runs

    `go test -v -timeout 15m CertificateService`


Some final notes. This was created in goland on a windows 10 Home machine. Theoretically it should
work in any platform, but I can't make any guarantees on this version.
