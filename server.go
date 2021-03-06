package main

import (
	"fmt"
	"net"
	"runtime"
	"sync"
	"net/rpc"
)

//-------------------------------------------------------------------------
// Daemon
//-------------------------------------------------------------------------

type Daemon struct {
	acr       *Server
	acc       *AutoCompleteContext
	pcache    PackageCache
	declcache *DeclCache
}

func NewDaemon(network, address string) *Daemon {
	d := new(Daemon)
	d.acr = NewServer(network, address)
	d.pcache = NewPackageCache()
	d.declcache = NewDeclCache()
	d.acc = NewAutoCompleteContext(d.pcache, d.declcache)
	return d
}

func (d *Daemon) DropCache() {
	d.pcache = NewPackageCache()
	d.declcache = NewDeclCache()
	d.acc = NewAutoCompleteContext(d.pcache, d.declcache)
}

var daemon *Daemon

//-------------------------------------------------------------------------
// printBacktrace
//-------------------------------------------------------------------------

var btSync sync.Mutex

func printBacktrace(err interface{}) {
	btSync.Lock()
	defer btSync.Unlock()
	fmt.Printf("panic: %v\n", err)
	i := 2
	for {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		f := runtime.FuncForPC(pc)
		fmt.Printf("%d(%s): %s:%d\n", i-1, f.Name(), file, line)
		i++
	}
	fmt.Println("")
}

//-------------------------------------------------------------------------
// Server_* functions
//
// Corresponding Client_* functions are autogenerated by goremote.
//-------------------------------------------------------------------------

func Server_AutoComplete(file []byte, filename string, cursor int) (a, b, c []string, d int) {
	defer func() {
		if err := recover(); err != nil {
			printBacktrace(err)
			a = []string{"PANIC"}
			b = a
			c = a

			// drop cache
			daemon.DropCache()
		}
	}()
	a, b, c, d = daemon.acc.Apropos(file, filename, cursor)
	return
}

func Server_Close(notused int) int {
	daemon.acr.Close()
	return 0
}

func Server_Status(notused int) string {
	return daemon.acc.Status()
}

func Server_DropCache(notused int) int {
	// drop cache
	daemon.DropCache()
	return 0
}

func Server_Set(key, value string) string {
	if key == "\x00" {
		return listConfig(&Config)
	} else if value == "\x00" {
		return listOption(&Config, key)
	}
	return setOption(&Config, key, value)
}

//-------------------------------------------------------------------------
// Server
//-------------------------------------------------------------------------

const (
	SERVER_CLOSE = iota
)

type Server struct {
	listener net.Listener
	cmd_in   chan int
}

func NewServer(network, address string) *Server {
	var err error

	s := new(Server)
	s.listener, err = net.Listen(network, address)
	if err != nil {
		panic(err.Error())
	}
	s.cmd_in = make(chan int, 1)
	return s
}

func acceptConnections(in chan net.Conn, listener net.Listener) {
	for {
		c, err := listener.Accept()
		if err != nil {
			panic(err.Error())
		}
		in <- c
	}
}

func (s *Server) Loop() {
	conn_in := make(chan net.Conn)
	go acceptConnections(conn_in, s.listener)
	for {
		// handle connections or server CMDs (currently one CMD)
		select {
		case c := <-conn_in:
			rpc.ServeConn(c)
			runtime.GC()
		case cmd := <-s.cmd_in:
			switch cmd {
			case SERVER_CLOSE:
				return
			}
		}
	}
}

func (s *Server) Close() {
	s.cmd_in <- SERVER_CLOSE
}
