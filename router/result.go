package router

// NoBody is the answer of a handler that carries none.
//
// A handler returns the thing that it answers. A handler that answers nothing
// returns NoBody, and the router writes 204 with no body.
//
//	func (m *Module) Delete(c *avero.Ctx, in DeleteInput) (avero.NoBody, error) {
//	    if err := m.store.Delete(c.Context(), in.ID); err != nil {
//	        return avero.NoBody{}, err
//	    }
//	    return avero.NoBody{}, nil
//	}
type NoBody struct{}
