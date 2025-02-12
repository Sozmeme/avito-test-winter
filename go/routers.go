package swagger

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

type Routes []Route

func NewRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	for _, route := range routes {
		var handler http.Handler
		handler = route.HandlerFunc
		handler = Logger(handler, route.Name)

		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(handler)
	}

	return router
}

func Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello World!")
}

var routes = Routes{
	Route{
		"Index",
		"GET",
		"/",
		Index,
	},

	Route{
		"ApiAuthPost",
		strings.ToUpper("Post"),
		"/api/auth",
		ApiAuthPost,
	},

	Route{
		"ApiBuyItemGet",
		strings.ToUpper("Get"),
		"/api/buy/{item}",
		ApiBuyItemGet,
	},

	Route{
		"ApiInfoGet",
		strings.ToUpper("Get"),
		"/api/info",
		ApiInfoGet,
	},

	Route{
		"ApiSendCoinPost",
		strings.ToUpper("Post"),
		"/api/sendCoin",
		ApiSendCoinPost,
	},
}
