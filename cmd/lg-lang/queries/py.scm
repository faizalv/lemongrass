; Top-level functions
(module
  (function_definition
    name: (identifier) @symbol
    parameters: (parameters) @params
    return_type: (_)? @return_type) @node)

; Top-level decorated functions
(module
  (decorated_definition
    definition: (function_definition
      name: (identifier) @symbol
      parameters: (parameters) @params
      return_type: (_)? @return_type)) @node)

; Classes
(module
  (class_definition
    name: (identifier) @symbol
    superclasses: (argument_list)? @superclasses) @node)

; Methods in classes
(module
  (class_definition
    name: (identifier) @receiver
    body: (block
      (function_definition
        name: (identifier) @symbol
        parameters: (parameters) @params
        return_type: (_)? @return_type) @node)))

; Methods in classes (decorated)
(module
  (class_definition
    name: (identifier) @receiver
    body: (block
      (decorated_definition
        definition: (function_definition
          name: (identifier) @symbol
          parameters: (parameters) @params
          return_type: (_)? @return_type)) @node)))
