package yogin

func MaxAllowed(n int) HandlerFunc {
	sem := make(chan struct{}, n)
	acquire := func() { sem <- struct{}{} }
	release := func() { <-sem }
	return func(c *Context) {
		acquire() // before request
		defer release() // after request
		c.Next()

	}
}
