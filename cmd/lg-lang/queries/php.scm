; Top-level functions
(program
  (function_definition
    name: (name) @symbol
    parameters: (formal_parameters) @params
    return_type: (_)? @return_type) @node)

; Classes (abstract_class_declaration does not exist -- abstract is a child modifier)
(program
  (class_declaration
    name: (name) @symbol
    (base_clause)? @extends
    (class_interface_clause)? @implements) @node)

; Interfaces
(program
  (interface_declaration
    name: (name) @symbol
    (base_clause)? @extends) @node)

; Traits
(program
  (trait_declaration
    name: (name) @symbol) @node)

; Enums
(program
  (enum_declaration
    name: (name) @symbol) @node)

; Methods in classes
(program
  (class_declaration
    name: (name) @receiver
    (declaration_list
      (method_declaration
        name: (name) @symbol
        parameters: (formal_parameters) @params
        return_type: (_)? @return_type) @node)))

; Methods in traits
(program
  (trait_declaration
    name: (name) @receiver
    (declaration_list
      (method_declaration
        name: (name) @symbol
        parameters: (formal_parameters) @params
        return_type: (_)? @return_type) @node)))

; Methods in interfaces
(program
  (interface_declaration
    name: (name) @receiver
    (declaration_list
      (method_declaration
        name: (name) @symbol
        parameters: (formal_parameters) @params
        return_type: (_)? @return_type) @node)))
