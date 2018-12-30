package twig

import (
	"context"
	"net/http"
	"os"
	"sync"
)

type M map[string]interface{}

type Attacher interface {
	Attach(*Twig)
}

type Identifier interface {
	Name() string
	Type() string
	Desc() string
}

type Twig struct {
	HttpErrorHandler HttpErrorHandler

	Logger  Logger
	Muxer   Muxer
	Servant Servant

	Debug bool

	pre []MiddlewareFunc
	mid []MiddlewareFunc

	pool sync.Pool
}

func TODO() *Twig {
	t := &Twig{
		Debug: false,
	}
	t.pool.New = func() interface{} {
		return t.NewCtx(nil, nil)
	}
	t.WithServant(NewDefaultServer(DefaultAddress)).
		WithHttpErrorHandler(DefaultHttpErrorHandler).
		WithLogger(newLog(os.Stdout, "twig-log-")).
		WithMux(NewRadixTreeMux())
	return t
}

func (t *Twig) Done() *Twig {
	if t.HttpErrorHandler == nil {
		panic("Twig: HttpErrorHandler is nil!")
	}

	if t.Logger == nil {
		panic("Twig: Logger is nil!")
	}

	if t.Muxer == nil {
		panic("Twig: Muxer is nil!")
	}

	if t.Servant == nil {
		panic("Twig: Servant is nil!")
	}
	return t
}

func (t *Twig) WithLogger(l Logger) *Twig {
	t.Logger = l
	return t
}

func (t *Twig) WithHttpErrorHandler(eh HttpErrorHandler) *Twig {
	t.HttpErrorHandler = eh
	return t
}

func (t *Twig) Pre(m ...MiddlewareFunc) *Twig {
	t.pre = append(t.pre, m...)
	return t
}

func (t *Twig) Use(m ...MiddlewareFunc) *Twig {
	t.mid = append(t.mid, m...)
	return t
}

func (t *Twig) WithMux(m Muxer) *Twig {
	t.Muxer = m
	m.Attach(t)
	return t
}

func (t *Twig) WithServant(s Servant) *Twig {
	t.Servant = s
	s.Attach(t)
	return t
}

func (t *Twig) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := t.pool.Get().(*ctx)
	c.Reset(w, r)

	h := Enhance(func(ctx Ctx) error {
		t.Muxer.Lookup(r.Method, GetReqPath(r), r, c)
		handler := Enhance(c.Handler(), t.mid)
		return handler(c)
	}, t.pre)

	if err := h(c); err != nil {
		t.HttpErrorHandler(err, c)
	}

	t.pool.Put(c)
}

func (t *Twig) Start() error {
	t.Logger.Println(banner)
	return t.Servant.Start()
}

func (t *Twig) Shutdown(ctx context.Context) error {
	return t.Servant.Shutdown(ctx)
}

func (t *Twig) NewCtx(w http.ResponseWriter, r *http.Request) Ctx {
	return &ctx{
		req:     r,
		resp:    NewResponseWarp(w),
		t:       t,
		store:   make(M),
		pvalues: make([]string, MaxParam),
		handler: NotFoundHandler,
	}
}

func (t *Twig) AcquireCtx() Ctx {
	c := t.pool.Get().(*ctx)
	return c
}

func (t *Twig) ReleaseCtx(c Ctx) {
	t.pool.Put(c)
}

func (t *Twig) add(method, path string, handler HandlerFunc, m ...MiddlewareFunc) *Route {
	return t.Muxer.Add(method, path, handler, m...)
}

func (t *Twig) Get(path string, handler HandlerFunc, m ...MiddlewareFunc) *Route {
	return t.add(GET, path, handler, m...)
}

func (t *Twig) Post(path string, handler HandlerFunc, m ...MiddlewareFunc) *Route {
	return t.add(POST, path, handler, m...)
}

func (t *Twig) Delete(path string, handler HandlerFunc, m ...MiddlewareFunc) *Route {
	return t.add(DELETE, path, handler, m...)
}

func (t *Twig) Put(path string, handler HandlerFunc, m ...MiddlewareFunc) *Route {
	return t.add(PUT, path, handler, m...)
}

func (t *Twig) Patch(path string, handler HandlerFunc, m ...MiddlewareFunc) *Route {
	return t.add(PATCH, path, handler, m...)
}

func (t *Twig) Head(path string, h HandlerFunc, m ...MiddlewareFunc) *Route {
	return t.add(HEAD, path, h, m...)
}

func (t *Twig) Options(path string, h HandlerFunc, m ...MiddlewareFunc) *Route {
	return t.add(OPTIONS, path, h, m...)
}
