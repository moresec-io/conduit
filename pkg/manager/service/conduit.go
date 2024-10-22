package service

type Conduit interface {
	SetClient()
	SetServer()
}

type conduit struct {
}
