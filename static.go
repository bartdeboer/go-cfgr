package cfgr

import "context"

// NoParams is the route parameter type used by static routes. It only needs to
// be named when passing a parameterless generic option to Register.
type NoParams struct{}

// StaticRoute is a route whose document does not depend on route parameters.
type StaticRoute struct {
	route *Route[NoParams]
}

// Register binds a static route to cfg. With no options it uses the default
// adapter, <name>.json, and allows all access.
func Register(cfg *Router, name string, options ...RouteOption[NoParams]) *StaticRoute {
	return &StaticRoute{route: RegisterAs(cfg, name, options...)}
}

func (r *StaticRoute) Name() string {
	return r.route.Name()
}

func (r *StaticRoute) ReadContents(ctx context.Context) ([]byte, error) {
	return r.route.ReadContents(ctx, NoParams{})
}

func (r *StaticRoute) WriteContents(ctx context.Context, contents []byte) error {
	return r.route.WriteContents(ctx, NoParams{}, contents)
}

func (r *StaticRoute) PatchContents(ctx context.Context, patch string) error {
	return r.route.PatchContents(ctx, NoParams{}, patch)
}

func (r *StaticRoute) Read(ctx context.Context, key string) (any, error) {
	return r.route.Read(ctx, NoParams{}, key)
}

func (r *StaticRoute) Write(ctx context.Context, key string, value any) error {
	return r.route.Write(ctx, NoParams{}, key, value)
}

func (r *StaticRoute) Unset(ctx context.Context, key string) error {
	return r.route.Unset(ctx, NoParams{}, key)
}

func (r *StaticRoute) List(ctx context.Context, key string) ([]Entry, error) {
	return r.route.List(ctx, NoParams{}, key)
}

func (r *StaticRoute) ReadString(ctx context.Context, key string) (string, error) {
	return r.route.ReadString(ctx, NoParams{}, key)
}

func (r *StaticRoute) ReadBool(ctx context.Context, key string) (bool, error) {
	return r.route.ReadBool(ctx, NoParams{}, key)
}

func (r *StaticRoute) ReadInt(ctx context.Context, key string) (int, error) {
	return r.route.ReadInt(ctx, NoParams{}, key)
}

func (r *StaticRoute) ReadFloat(ctx context.Context, key string) (float64, error) {
	return r.route.ReadFloat(ctx, NoParams{}, key)
}

func (r *StaticRoute) ReadInto(ctx context.Context, key string, dst any) error {
	return r.route.ReadInto(ctx, NoParams{}, key, dst)
}
