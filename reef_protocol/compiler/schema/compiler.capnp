using Go = import "/go.capnp";

@0xcf0367ee3bec7bc8;

$Go.package("message");
$Go.import("foo/message");

struct CompilerResponse {
    union {
        fileContent @0 :Data;
        compilerError @1 :Text;
        systemError @2 :Text;
    }
}

enum Language {
    c @0;
    rust @1;
}

interface Compiler {
    compile @0 (programSrc :Text, language :Language) -> (response :CompilerResponse);
}
