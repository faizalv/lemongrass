; Script block (setup or legacy -- Go checks for "setup" attr in raw bytes)
(script_element
  (raw_text) @script_text) @script_block

; Template block
(template_element) @template_block

; Style block
(style_element
  (raw_text)? @style_text) @style_block
