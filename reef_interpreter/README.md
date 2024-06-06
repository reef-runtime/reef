# reef_interpreter

A Wasm interpreter forked of [TinyWasm](https://github.com/explodingcamera/tinywasm).
A significant amount simplifications have been made to remove features not required for reef, mainly multi-module linking.
Further modifications have been made to allow execution of Wasm to be halted and execution state serialized.
