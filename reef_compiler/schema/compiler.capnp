@0xcf0367ee3bec7bc8;

struct CompilerResponse {
    union {
        fileContent @0 :Data;
        compilerError @1 :Text;
        fileSystemError @2 :Text;
    }
}

enum Language {
    c @0;
    rust @1;
}

interface Compiler {
    struct Config {
        mainPath @0 :Text;
        depPath @1 :Text;
    }

    compile @0 (programSrc :Text, language :Language) -> (response :CompilerResponse);
}
