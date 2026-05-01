package api

schema_version: "v1"

api: {
	base_path: "/"
}

info: {
	title:       "APIGen Todo Example"
	version:     "0.1.0"
	description: "Small in-memory todo API authored in CUE to showcase APIGen generation and strict server wiring."
}

servers: [
	{
		url:         "http://127.0.0.1:8081"
		description: "Example development server"
		variables:   {}
	},
]

tags: [
	{
		name:        "Todos"
		description: "Todo lifecycle endpoints for the APIGen example."
	},
]

openapi: {
	version: "3.0.0"
	tag_order: ["Todos"]
	security_schemes: {
		BearerAuth: {
			type:   "http"
			scheme: "Bearer"
		}
		ApiKeyAuth: {
			type: "apiKey"
			in:   "header"
			name: "X-API-Key"
		}
	}
}
