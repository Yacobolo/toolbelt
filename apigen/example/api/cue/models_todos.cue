package api

schemas_todos: {
	Todo: {
		type: "object"
		properties: {
			id: {
				schema: {
					type: "string"
				}
			}
			title: {
				schema: {
					type: "string"
				}
			}
			status: {
				schema: {
					type: "string"
				}
			}
		}
		required: ["id", "title", "status"]
	}
	CreateTodoRequest: {
		type: "object"
		properties: {
			title: {
				schema: {
					type: "string"
				}
			}
		}
		required: ["title"]
	}
	ListTodosResponse: {
		type: "object"
		properties: {
			data: {
				schema: {
					type: "array"
					items: {
						ref: "Todo"
					}
				}
			}
		}
		required: ["data"]
	}
	Error: {
		type: "object"
		properties: {
			code: {
				schema: {
					type: "integer"
					format: "int32"
				}
			}
			message: {
				schema: {
					type: "string"
				}
			}
		}
		required: ["code", "message"]
	}
}
