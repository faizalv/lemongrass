; Exported function declarations
(export_statement
  declaration: (function_declaration
    name: (identifier) @symbol
    parameters: (formal_parameters) @params
    return_type: (_)? @return_type) @node) @export

(export_statement
  declaration: (generator_function_declaration
    name: (identifier) @symbol
    parameters: (formal_parameters) @params) @node) @export

; Exported classes
(export_statement
  declaration: (class_declaration
    name: (type_identifier) @symbol
    (class_heritage)? @heritage) @node) @export

(export_statement
  declaration: (abstract_class_declaration
    name: (type_identifier) @symbol
    (class_heritage)? @heritage) @node) @export

; Exported interfaces
(export_statement
  declaration: (interface_declaration
    name: (type_identifier) @symbol
    (extends_type_clause)? @extends) @node) @export

; Exported type aliases
(export_statement
  declaration: (type_alias_declaration
    name: (type_identifier) @symbol) @node) @export

; Exported enums
(export_statement
  declaration: (enum_declaration
    name: (identifier) @symbol) @node) @export

; Exported const/let with value
(export_statement
  declaration: (lexical_declaration
    (variable_declarator
      name: (identifier) @symbol
      value: (_) @value) @node)) @export

; Bare function (Vue script setup -- no export keyword)
(function_declaration
  name: (identifier) @symbol
  parameters: (formal_parameters) @params
  return_type: (_)? @return_type) @node

; Bare const arrow function (Vue script setup)
(lexical_declaration
  (variable_declarator
    name: (identifier) @symbol
    value: (arrow_function) @value) @node)

(lexical_declaration
  (variable_declarator
    name: (identifier) @symbol
    value: (function_expression) @value) @node)
