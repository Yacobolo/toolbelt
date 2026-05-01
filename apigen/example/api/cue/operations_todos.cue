package api

endpoints_todos: [
	{
		method:       "get"
		path:         "/todos"
		operation_id: "listTodos"
		summary:      "List todos"
		tags:         ["Todos"]
		parameters: [
			{
				name:        "status"
				in:          "query"
				description: "Optional status filter."
				schema: {
					type: "string"
				}
			},
		]
		responses: [
			{
				status_code: 200
				description: "The request has succeeded."
				schema: {
					ref: "ListTodosResponse"
				}
			},
			{
				status_code: 400
				description: "The request is invalid."
				schema: {
					ref: "Error"
				}
			},
		]
		cli: {
			command: ["todos", "list"]
			output: {
				mode: "collection"
				table_columns: ["id", "title", "status"]
				quiet_fields: ["id"]
			}
		}
	},
	{
		method:       "post"
		path:         "/todos"
		operation_id: "createTodo"
		summary:      "Create todo"
		tags:         ["Todos"]
		request_body: {
			required: true
			schema: {
				ref: "CreateTodoRequest"
			}
		}
		responses: [
			{
				status_code: 201
				description: "A todo has been created."
				schema: {
					ref: "Todo"
				}
			},
			{
				status_code: 400
				description: "The request is invalid."
				schema: {
					ref: "Error"
				}
			},
		]
		cli: {
			command: ["todos", "create"]
			body_input: "flags_or_json"
			args: [{
				source: "body"
				name: "title"
				display_name: "title"
			}]
			output: {
				mode: "detail"
				quiet_fields: ["id"]
			}
		}
	},
	{
		method:       "get"
		path:         "/todos/{todo_id}"
		operation_id: "getTodo"
		summary:      "Get todo"
		tags:         ["Todos"]
		parameters: [
			{
				name:     "todo_id"
				in:       "path"
				required: true
				schema: {
					type: "string"
				}
			},
		]
		responses: [
			{
				status_code: 200
				description: "The request has succeeded."
				schema: {
					ref: "Todo"
				}
			},
			{
				status_code: 404
				description: "The todo was not found."
				schema: {
					ref: "Error"
				}
			},
		]
		cli: {
			command: ["todos", "get"]
			args: [{
				source: "path"
				name: "todo_id"
				display_name: "todo-id"
			}]
			output: {
				mode: "detail"
				quiet_fields: ["id"]
			}
		}
	},
	{
		method:       "post"
		path:         "/todos/{todo_id}/complete"
		operation_id: "completeTodo"
		summary:      "Complete todo"
		tags:         ["Todos"]
		parameters: [
			{
				name:     "todo_id"
				in:       "path"
				required: true
				schema: {
					type: "string"
				}
			},
		]
		responses: [
			{
				status_code: 200
				description: "The todo has been completed."
				schema: {
					ref: "Todo"
				}
			},
			{
				status_code: 404
				description: "The todo was not found."
				schema: {
					ref: "Error"
				}
			},
		]
		cli: {
			command: ["todos", "complete"]
			args: [{
				source: "path"
				name: "todo_id"
				display_name: "todo-id"
			}]
			output: {
				mode: "detail"
				quiet_fields: ["id"]
			}
		}
	},
	{
		method:       "delete"
		path:         "/todos/{todo_id}"
		operation_id: "deleteTodo"
		summary:      "Delete todo"
		tags:         ["Todos"]
		parameters: [
			{
				name:     "todo_id"
				in:       "path"
				required: true
				schema: {
					type: "string"
				}
			},
		]
		responses: [
			{
				status_code: 204
				description: "The todo has been deleted."
			},
			{
				status_code: 404
				description: "The todo was not found."
				schema: {
					ref: "Error"
				}
			},
		]
		cli: {
			command: ["todos", "delete"]
			args: [{
				source: "path"
				name: "todo_id"
				display_name: "todo-id"
			}]
			confirm: "always"
			output: {
				mode: "empty"
			}
		}
	},
]
