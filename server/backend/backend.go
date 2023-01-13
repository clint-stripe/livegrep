package backend

import (
	"context"
	"sync"
	"time"

	pb "github.com/livegrep/livegrep/src/proto/go_proto"
)

type Tree struct {
	Name    string
	Version string
	Url     string
}

type I struct {
	Name  string
	Trees []Tree
	sync.Mutex
	IndexTime time.Time
}

type Searchable interface {
	Start()
	Search(context.Context, *pb.Query) (*pb.CodeSearchResult, error)
}

type Backend struct {
	Id   string
	Addr string
	I    *I

	Searchable
}
