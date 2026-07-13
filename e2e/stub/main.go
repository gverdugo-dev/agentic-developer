// Command stub is the e2e suite's fake skills.sh: a tiny static file server.
// It serves the fixture registry tree (run.sh points ADEV_REGISTRY_URL and
// ADEV_REGISTRY_TARBALL_URL at it), so the explore flow never touches the
// network:
//
//	<dir>/api/search                     the canned search response (the
//	                                     query string is ignored, like any
//	                                     static server ignores it)
//	<dir>/<owner>/<repo>/tar.gz/HEAD     the canned repo tarball
//
// It listens on an OS-chosen loopback port and writes that port to the
// -portfile, which is how run.sh learns where it landed.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
)

func main() {
	dir := flag.String("dir", "", "directory tree to serve")
	portFile := flag.String("portfile", "", "file to write the chosen port to")
	flag.Parse()
	if *dir == "" || *portFile == "" {
		log.Fatal("usage: stub -dir <tree> -portfile <file>")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := os.WriteFile(*portFile, []byte(fmt.Sprintf("%d", port)), 0o644); err != nil {
		log.Fatal(err)
	}

	log.Fatal(http.Serve(listener, http.FileServer(http.Dir(*dir))))
}
