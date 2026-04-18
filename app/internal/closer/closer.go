package closer

import (
	"context"
	"errors"
	"sync"
)

type closeFn struct {
	name string
	fn   func(context.Context) error
}

type Closer struct {
	m    sync.Mutex
	once sync.Once
	fns  []closeFn
}

func New() *Closer {
	return &Closer{}
}

func (c *Closer) Add(name string, fn func(context.Context) error) {
	c.m.Lock()
	defer c.m.Unlock()

	c.fns = append(c.fns, closeFn{name, fn})
}

func (c *Closer) CloseAll(ctx context.Context) error {
	var err error

	c.once.Do(func() {
		c.m.Lock()
		fns := c.fns
		c.fns = nil
		c.m.Unlock()

		if len(fns) == 0 {
			return
		}

		var errs []error

		for i := len(fns) - 1; i >= 0; i-- {
			if err := fns[i].fn(ctx); err != nil {
				errs = append(errs, err)
			}
		}

		err = errors.Join(errs...)
	})

	return err
}
