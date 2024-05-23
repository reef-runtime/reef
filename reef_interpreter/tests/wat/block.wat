(module
  (func $block_br
    (block $block1 
      i32.const 1
      drop
      br $block1
      i32.const 1
      drop
    )
  )
  (export "block_br" (func $block_br))
)
