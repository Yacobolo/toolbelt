package api

schema_version: "v1"

api: {
	base_path: "/v1"
}

info: {
	title:   "Lineage API"
	version: "0.1.0"
}

openapi: {
	version: "3.0.0"
}

schemas: {
	"LineageNode": {
		type: "object"
		properties: {
			"table_name": {
				schema: {
					type: "string"
				}
			}
			"upstream": {
				schema: {
					type: "array"
					items: {
						ref: "LineageEdge"
					}
				}
			}
			"downstream": {
				schema: {
					type: "array"
					items: {
						ref: "LineageEdge"
					}
				}
			}
		}
		required: ["table_name", "upstream", "downstream"]
	}
	"LineageEdge": {
		type: "object"
		properties: {
			"source": {
				schema: {
					type: "string"
				}
			}
			"target": {
				schema: {
					type: "string"
				}
			}
		}
		required: ["source", "target"]
	}
	"PaginatedLineageEdges": {
		type: "object"
		properties: {
			"data": {
				schema: {
					type: "array"
					items: {
						ref: "LineageEdge"
					}
				}
			}
			"next_page_token": {
				schema: {
					type: "string"
				}
			}
		}
		required: ["data"]
	}
	"APIKeyInfo": {
		type: "object"
		properties: {
			"id": {
				schema: {
					type: "string"
				}
			}
			"principal_id": {
				schema: {
					type: "string"
				}
			}
			"name": {
				schema: {
					type: "string"
				}
			}
			"key_prefix": {
				schema: {
					type: "string"
				}
			}
			"expires_at": {
				schema: {
					type:   "string"
					format: "date-time"
				}
			}
			"created_at": {
				schema: {
					type:   "string"
					format: "date-time"
				}
			}
		}
		required: ["id", "principal_id", "name", "key_prefix", "created_at"]
	}
	"PurgeLineageRequest": {
		type: "object"
		properties: {
			"table_name": {
				schema: {
					type: "string"
				}
			}
			"delete_descendants": {
				schema: {
					type: "boolean"
				}
			}
		}
		required: ["table_name"]
	}
}

endpoints: [
	{
		method:       "get"
		path:         "/lineage/{schema_name}/{table_name}"
		operation_id: "getTableLineage"
		parameters: [
			{
				name:     "schema_name"
				in:       "path"
				required: true
				schema: {
					type: "string"
				}
			},
			{
				name:     "table_name"
				in:       "path"
				required: true
				schema: {
					type: "string"
				}
			},
			{
				name:    "max_results"
				in:      "query"
				explode: false
				schema: {
					type: "integer"
				}
			},
			{
				name:    "page_token"
				in:      "query"
				explode: false
				schema: {
					type: "string"
				}
			},
		]
		responses: [
			{
				status_code: 200
				description: "ok"
				schema: {
					ref: "PaginatedLineageEdges"
				}
			},
		]
		},
		{
		method:       "post"
		path:         "/lineage/purge"
		operation_id: "purgeLineage"
		request_body: {
			required: true
			schema: {
				ref: "PurgeLineageRequest"
			}
		}
		responses: [
			{
				status_code: 201
				description: "accepted"
				schema: {
					ref: "APIKeyInfo"
				}
			},
		]
		},
	]
